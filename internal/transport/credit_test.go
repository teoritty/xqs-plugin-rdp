package transport_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/transport"
)

func TestSendWindowSpendsInitialCredit(t *testing.T) {
	w := transport.NewSendWindow(4)
	for i := 0; i < 4; i++ {
		if err := w.Acquire(context.Background()); err != nil {
			t.Fatalf("Acquire #%d: %v", i, err)
		}
	}
	if got := w.Available(); got != 0 {
		t.Fatalf("Available = %d, ожидался 0", got)
	}
}

// Отправка сверх выданного кредита убивает процесс на стороне ядра, поэтому
// на нуле отправитель обязан ЖДАТЬ, а не проскакивать.
func TestSendWindowBlocksAtZeroAndWakesOnGrant(t *testing.T) {
	w := transport.NewSendWindow(1)
	if err := w.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	acquired := make(chan error, 1)
	go func() { acquired <- w.Acquire(context.Background()) }()

	select {
	case err := <-acquired:
		t.Fatalf("Acquire на нулевом кредите не заблокировался: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	w.Grant(2)
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatalf("Acquire после пополнения: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Acquire не проснулся после пополнения")
	}
	if got := w.Available(); got != 1 {
		t.Fatalf("Available = %d, ожидался 1 (выдали 2, потратили 1)", got)
	}
}

// Отмена контекста обязана будить ожидающего: иначе закрытие сессии повисает
// на отправителе, который ждёт кредита, которого уже никто не выдаст.
func TestSendWindowRespectsContextCancellation(t *testing.T) {
	w := transport.NewSendWindow(0)
	ctx, cancel := context.WithCancel(context.Background())

	acquired := make(chan error, 1)
	go func() { acquired <- w.Acquire(ctx) }()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-acquired:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, ожидался context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Acquire не проснулся на отмене контекста")
	}
}

func TestSendWindowCloseWakesWaiters(t *testing.T) {
	w := transport.NewSendWindow(0)
	acquired := make(chan error, 1)
	go func() { acquired <- w.Acquire(context.Background()) }()
	time.Sleep(20 * time.Millisecond)
	w.Close()

	select {
	case err := <-acquired:
		if !errors.Is(err, transport.ErrClosed) {
			t.Fatalf("err = %v, ожидалась ErrClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Acquire не проснулся на закрытии окна")
	}
	if err := w.Acquire(context.Background()); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("Acquire на закрытом окне: %v", err)
	}
}

func TestSendWindowIsSafeUnderConcurrency(t *testing.T) {
	const grants = 200
	w := transport.NewSendWindow(0)

	var wg sync.WaitGroup
	wg.Add(grants)
	for i := 0; i < grants; i++ {
		go func() {
			defer wg.Done()
			if err := w.Acquire(context.Background()); err != nil {
				t.Errorf("Acquire: %v", err)
			}
		}()
	}
	for i := 0; i < grants; i++ {
		w.Grant(1)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("не все отправители получили кредит")
	}
	if got := w.Available(); got != 0 {
		t.Fatalf("Available = %d, ожидался 0", got)
	}
}

// Приёмная половина. Она интереснее исходящей: удержание грантов — наш
// единственный рычаг давления на источник.
func TestRecvWindowGrantsAsDataIsConsumed(t *testing.T) {
	var granted []int
	r := transport.NewRecvWindow(8, func(n int) { granted = append(granted, n) })

	// Пока ничего не потреблено, гранты не выдаются.
	if len(granted) != 0 {
		t.Fatalf("гранты выданы до потребления: %v", granted)
	}
	for i := 0; i < 8; i++ {
		r.Consumed(1)
	}
	total := 0
	for _, n := range granted {
		total += n
	}
	if total != 8 {
		t.Fatalf("выдано %d кредитов, потреблено 8", total)
	}
}

// Ключевое свойство для связки окон: пока гранты удержаны, они НЕ уходят
// хосту, и источник останавливается. Это то, чем tcp-relay регулируется по
// состоянию embed-stream.
func TestRecvWindowHoldSuppressesGrantsAndReleaseFlushesThem(t *testing.T) {
	var granted []int
	r := transport.NewRecvWindow(8, func(n int) { granted = append(granted, n) })

	r.Hold()
	for i := 0; i < 5; i++ {
		r.Consumed(1)
	}
	if len(granted) != 0 {
		t.Fatalf("во время удержания выданы гранты: %v", granted)
	}

	r.Release()
	total := 0
	for _, n := range granted {
		total += n
	}
	if total != 5 {
		t.Fatalf("после освобождения выдано %d, ожидалось 5", total)
	}
}

// Удержание идемпотентно и вложенно: связка может удерживать окно из
// нескольких мест, и одно освобождение не должно отпускать давление целиком.
func TestRecvWindowHoldIsNested(t *testing.T) {
	var granted []int
	r := transport.NewRecvWindow(8, func(n int) { granted = append(granted, n) })

	r.Hold()
	r.Hold()
	r.Consumed(3)
	r.Release()
	if len(granted) != 0 {
		t.Fatalf("одно освобождение из двух отпустило давление: %v", granted)
	}
	r.Release()
	if len(granted) == 0 {
		t.Fatal("после парного освобождения гранты не выданы")
	}
}

// Лишнее освобождение — дефект вызывающего, и он не должен превращаться в
// «удержание ушло в минус», из-за которого окно потом не удержится вовсе.
func TestRecvWindowExtraReleaseDoesNotUnderflow(t *testing.T) {
	var granted []int
	r := transport.NewRecvWindow(8, func(n int) { granted = append(granted, n) })

	r.Release()
	r.Release()
	r.Hold()
	r.Consumed(2)
	if len(granted) != 0 {
		t.Fatalf("удержание не сработало после лишних освобождений: %v", granted)
	}
}

// Held — то, по чему связка окон принимает решения, а Capacity — то, с чем она
// сравнивает остаток. Обе обязаны говорить правду, иначе давление будет
// прикладываться не тогда, когда нужно.
func TestRecvWindowReportsHoldStateAndCapacity(t *testing.T) {
	r := transport.NewRecvWindow(8, func(int) {})
	if r.Capacity() != 8 {
		t.Fatalf("Capacity = %d, ожидалось 8", r.Capacity())
	}
	if r.Held() {
		t.Fatal("новое окно не должно быть удержано")
	}
	r.Hold()
	if !r.Held() {
		t.Fatal("Held не отражает удержание")
	}
	r.Release()
	if r.Held() {
		t.Fatal("Held не сбросился после освобождения")
	}
}

// Окно без получателя грантов не должно падать: так его удобно использовать в
// тестах и в путях, где подтверждать некому.
func TestRecvWindowToleratesNilGrantFunc(t *testing.T) {
	r := transport.NewRecvWindow(4, nil)
	r.Consumed(2)
	r.Hold()
	r.Consumed(1)
	r.Release()
}
