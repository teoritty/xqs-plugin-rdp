package relay_test

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"xqs-plugin-rdp/internal/relay"
)

// fakeWindow фиксирует не только текущее состояние, но и историю: связку важно
// проверять по тому, КОГДА приложено давление, а не только по тому, приложено
// ли оно в конце.
type fakeWindow struct {
	mu     sync.Mutex
	holds  int
	events []string
}

func (w *fakeWindow) Hold() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.holds++
	w.events = append(w.events, "hold")
}

func (w *fakeWindow) Release() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.holds--
	w.events = append(w.events, "release")
}

func (w *fakeWindow) Held() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.holds > 0
}

func (w *fakeWindow) history() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.events...)
}

func TestCouplingHoldsUpstreamOnPause(t *testing.T) {
	w := &fakeWindow{}
	c := relay.NewCoupling(w)

	if c.Paused() {
		t.Fatal("новая связка не должна удерживать окно")
	}
	c.Pause()
	if !w.Held() {
		t.Fatal("Pause не удержал окно к серверу — давление не дойдёт до источника")
	}
	if !c.Paused() {
		t.Fatal("Paused не отражает состояние")
	}
	c.Resume()
	if w.Held() {
		t.Fatal("Resume не снял удержание")
	}
}

// Браузер может сообщать о заторе многократно. Каждое сообщение не должно
// добавлять по удержанию: снять их все потом станет нечем, и труба к серверу
// останется перекрытой навсегда.
func TestCouplingIsIdempotent(t *testing.T) {
	w := &fakeWindow{}
	c := relay.NewCoupling(w)

	c.Pause()
	c.Pause()
	c.Pause()
	c.Resume()

	if w.Held() {
		t.Fatalf("после трёх Pause и одного Resume окно осталось удержанным: %v", w.history())
	}
	if got := w.history(); len(got) != 2 || got[0] != "hold" || got[1] != "release" {
		t.Fatalf("история %v, ожидались ровно hold и release", got)
	}
}

// Лишний Resume не должен уводить удержание в минус: тогда следующий Pause не
// сработал бы, и связка перестала бы работать молча.
func TestCouplingExtraResumeIsHarmless(t *testing.T) {
	w := &fakeWindow{}
	c := relay.NewCoupling(w)

	c.Resume()
	c.Resume()
	c.Pause()
	if !w.Held() {
		t.Fatalf("Pause не сработал после лишних Resume: %v", w.history())
	}
}

func TestCouplingIsSafeUnderConcurrency(t *testing.T) {
	w := &fakeWindow{}
	c := relay.NewCoupling(w)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); c.Pause() }()
		go func() { defer wg.Done(); c.Resume() }()
	}
	wg.Wait()
	c.Resume()
	if w.Held() {
		t.Fatal("после финального Resume окно осталось удержанным")
	}
}

func TestPumpMovesBytesBothWays(t *testing.T) {
	browserOur, browserFar := net.Pipe()
	serverOur, serverFar := net.Pipe()

	done := make(chan error, 1)
	go func() { done <- relay.Pump(browserOur, serverOur) }()

	// Браузер → сервер.
	go func() { _, _ = browserFar.Write([]byte("to-server")) }()
	got := make([]byte, len("to-server"))
	if err := serverFar.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if _, err := io.ReadFull(serverFar, got); err != nil {
		t.Fatalf("сервер не получил: %v", err)
	}
	if !bytes.Equal(got, []byte("to-server")) {
		t.Fatalf("сервер получил %q", got)
	}

	// Сервер → браузер.
	go func() { _, _ = serverFar.Write([]byte("to-browser")) }()
	got = make([]byte, len("to-browser"))
	if err := browserFar.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	if _, err := io.ReadFull(browserFar, got); err != nil {
		t.Fatalf("браузер не получил: %v", err)
	}
	if !bytes.Equal(got, []byte("to-browser")) {
		t.Fatalf("браузер получил %q", got)
	}

	_ = browserFar.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Pump не завершился после закрытия браузера")
	}
}

// Закрытие любой из сторон обязано завершать обе: половина живой сессии
// бесполезна, а оставленная горутина пережила бы саму сессию.
func TestPumpStopsWhenEitherSideCloses(t *testing.T) {
	for _, closeServer := range []bool{false, true} {
		t.Run(map[bool]string{false: "браузер", true: "сервер"}[closeServer], func(t *testing.T) {
			browserOur, browserFar := net.Pipe()
			serverOur, serverFar := net.Pipe()

			done := make(chan error, 1)
			go func() { done <- relay.Pump(browserOur, serverOur) }()

			if closeServer {
				_ = serverFar.Close()
			} else {
				_ = browserFar.Close()
			}

			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("нормальное закрытие превратилось в ошибку: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Pump не завершился")
			}

			// Обе стороны обязаны быть закрыты.
			if _, err := browserFar.Write([]byte("x")); err == nil {
				t.Error("сторона браузера осталась открытой")
			}
			if _, err := serverFar.Write([]byte("x")); err == nil {
				t.Error("сторона сервера осталась открытой")
			}
		})
	}
}
