package worker

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

// sendLoopUltra - 100% 달성을 위한 초고성능 모드
func (w *UDPWorker) sendLoopUltra(ctx context.Context) {
	// 연결 상태 확인
	if w.conn == nil {
		fmt.Printf("Worker %d: ERROR - UDP connection is nil!\n", w.ID)
		return
	}
	
	// CPU 코어에 고정 (Linux only)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	
	targetEPS := w.targetEPS
	if targetEPS == 0 {
		targetEPS = 25000
	}
	
	// 나노초 단위 정밀 타이밍
	intervalNanos := int64(10_000_000) // 10ms in nanoseconds
	logsPerBatch := int64(targetEPS * 10 / 1000) // 10ms당 로그 수
	
	// 100% 달성을 위한 정확한 보정 (94% -> 100% = 1.064x)
	logsPerBatch = int64(float64(logsPerBatch) * 1.064)
	
	fmt.Printf("Worker %d: Ultra mode - %d logs every 10ms = %d EPS (target: %d)\n",
		w.ID, logsPerBatch, logsPerBatch*100, targetEPS)
	
	// 배치 버퍼 사전 할당
	maxBatch := int(logsPerBatch * 2)
	preallocBuffer := make([][]byte, 0, maxBatch)
	w.batchBuffer = preallocBuffer
	
	// 시작 시간
	nextSendTime := time.Now().UnixNano()
	lastAdjustTime := nextSendTime
	totalSentInWindow := int64(0)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		default:
			// 배치 생성
			w.batchBuffer = w.batchBuffer[:0]
			for i := int64(0); i < logsPerBatch; i++ {
				log := w.generator.GenerateSystemLog()
				w.batchBuffer = append(w.batchBuffer, log)
			}
			
			// 전송
			err := w.sendBatch()
			if err != nil {
				w.errorCount.Add(1)
			} else {
				sent := logsPerBatch
				w.totalSent.Add(sent)
				totalSentInWindow += sent
			}
			
			// 다음 전송 시간 계산
			nextSendTime += intervalNanos
			
			// Busy-wait 기반 정밀 대기
			for {
				now := time.Now().UnixNano()
				if now >= nextSendTime {
					break
				}
				
				remaining := nextSendTime - now
				if remaining > 1_000_000 { // 1ms 이상 남았으면
					// Sleep으로 대부분 대기
					time.Sleep(time.Duration(remaining - 500_000)) // 500us 전까지 sleep
				} else if remaining > 10_000 { // 10us 이상 남았으면
					// Yield로 CPU 양보
					runtime.Gosched()
				}
				// 10us 미만은 busy-wait
			}
			
			// 100ms마다 동적 조정
			currentTime := time.Now().UnixNano()
			if currentTime - lastAdjustTime >= 100_000_000 { // 100ms
				elapsed := float64(currentTime - lastAdjustTime) / 1e9
				actualEPS := float64(totalSentInWindow) / elapsed
				targetFloat := float64(targetEPS)
				
				// 오차 계산 및 보정
				errorRate := (targetFloat - actualEPS) / targetFloat
				if errorRate > 0.01 { // 1% 미만이면 증가
					logsPerBatch = int64(float64(logsPerBatch) * 1.01)
				} else if errorRate < -0.01 { // 1% 초과면 감소
					logsPerBatch = int64(float64(logsPerBatch) * 0.99)
				}
				
				// 리셋
				totalSentInWindow = 0
				lastAdjustTime = currentTime
			}
		}
	}
}

// sendLoopRealtime - 실시간 스케줄링 우선순위 모드 (Linux only)
func (w *UDPWorker) sendLoopRealtime(ctx context.Context) {
	// CPU 코어 고정
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	
	targetEPS := w.targetEPS
	if targetEPS == 0 {
		targetEPS = 25000
	}
	
	// 배치 크기 계산 (10ms 간격, 100% 보정 적용)
	logsPerBatch := int64(float64(targetEPS) * 0.01064) // 10ms * 1.064 보정
	
	fmt.Printf("Worker %d: Realtime mode - %d logs/10ms = %d EPS\n",
		w.ID, logsPerBatch, logsPerBatch*100)
	
	// 고정밀 타이머 생성
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	
	// 더블 버퍼링
	buffer1 := make([][]byte, 0, logsPerBatch)
	buffer2 := make([][]byte, 0, logsPerBatch)
	currentBuffer := &buffer1
	nextBuffer := &buffer2
	
	// 백그라운드 생성 고루틴
	genChan := make(chan [][]byte, 2)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// 다음 배치 미리 생성
				batch := make([][]byte, 0, logsPerBatch)
				for i := int64(0); i < logsPerBatch; i++ {
					log := w.generator.GenerateSystemLog()
					batch = append(batch, log)
				}
				genChan <- batch
			}
		}
	}()
	
	// 첫 배치 준비
	*currentBuffer = <-genChan
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopChan:
			return
		case <-ticker.C:
			// 현재 배치 전송
			w.batchBuffer = *currentBuffer
			err := w.sendBatch()
			if err != nil {
				w.errorCount.Add(1)
			} else {
				w.totalSent.Add(logsPerBatch)
			}
			
			// 버퍼 스왑
			currentBuffer, nextBuffer = nextBuffer, currentBuffer
			
			// 다음 배치 준비 (논블로킹)
			select {
			case *currentBuffer = <-genChan:
			default:
				// 생성이 늦으면 직접 생성
				(*currentBuffer) = (*currentBuffer)[:0]
				for i := int64(0); i < logsPerBatch; i++ {
					log := w.generator.GenerateSystemLog()
					*currentBuffer = append(*currentBuffer, log)
				}
			}
		}
	}
}