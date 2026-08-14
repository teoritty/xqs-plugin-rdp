package session

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"xqs-plugin-rdp/internal/proxy"
)

// Ядру уходит ровно то, что оно ждёт: идентификатор сессии, адрес и материал
// в base64. Ошибка в любом из трёх означает либо отказ ядра, либо доверие,
// записанное не к тому узлу.
func TestVerifyPeerSendsWhatTheCoreExpects(t *testing.T) {
	host := newFakeHost()
	host.results["trust.verifyPeer"] = map[string]bool{"trusted": true}
	m := NewManager(host, &fakeChannels{})

	material := []byte{0x30, 0x59, 0x00, 0xFF}
	trusted, err := m.verifyPeer(context.Background(), "s-1", "10.0.0.5:3389", material)
	if err != nil {
		t.Fatalf("verifyPeer: %v", err)
	}
	if !trusted {
		t.Fatal("ответ ядра trusted:true не доехал")
	}

	calls := host.callsTo("trust.verifyPeer")
	if len(calls) != 1 {
		t.Fatalf("вызовов %d, ожидался 1", len(calls))
	}
	p, ok := calls[0].params.(verifyPeerParams)
	if !ok {
		t.Fatalf("параметры типа %T", calls[0].params)
	}
	if p.SessionID != "s-1" {
		t.Fatalf("sessionId = %q", p.SessionID)
	}
	if p.Subject != "10.0.0.5:3389" {
		t.Fatalf("subject = %q", p.Subject)
	}
	got, err := base64.StdEncoding.DecodeString(p.MaterialBase64)
	if err != nil {
		t.Fatalf("материал не декодируется: %v", err)
	}
	if string(got) != string(material) {
		t.Fatalf("материал искажён: % x", got)
	}
}

// Ответ «не доверяем» — это не ошибка вызова. Ядро в этот момент уже задало
// вопрос пользователю, и превращать его в сбой значило бы затереть вопрос.
func TestVerifyPeerReportsRefusalWithoutError(t *testing.T) {
	host := newFakeHost()
	host.results["trust.verifyPeer"] = map[string]bool{"trusted": false}
	m := NewManager(host, &fakeChannels{})

	trusted, err := m.verifyPeer(context.Background(), "s-1", "h:3389", []byte{1})
	if err != nil {
		t.Fatalf("отказ выдан за ошибку: %v", err)
	}
	if trusted {
		t.Fatal("отказ прочитан как доверие")
	}
}

// Сбой шины — не «доверяем». Отсутствие ответа не повод ответить самому.
func TestVerifyPeerRefusesWhenTheCallFails(t *testing.T) {
	host := newFakeHost()
	boom := errors.New("шина недоступна")
	host.failOn["trust.verifyPeer"] = boom
	m := NewManager(host, &fakeChannels{})

	trusted, err := m.verifyPeer(context.Background(), "s-1", "h:3389", []byte{1})
	if trusted {
		t.Fatal("сбой вызова прочитан как доверие")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидалась причина сбоя", err)
	}
}

// Ядро ответило без поля trusted — читаем это как «не доверяем», а не как
// доверие по умолчанию.
func TestVerifyPeerTreatsAnEmptyAnswerAsRefusal(t *testing.T) {
	host := newFakeHost()
	host.results["trust.verifyPeer"] = map[string]any{}
	m := NewManager(host, &fakeChannels{})

	trusted, err := m.verifyPeer(context.Background(), "s-1", "h:3389", []byte{1})
	if err != nil {
		t.Fatalf("verifyPeer: %v", err)
	}
	if trusted {
		t.Fatal("ответ без поля trusted прочитан как доверие")
	}
}

// САМОЕ ВАЖНОЕ свойство переноса доверия в ядро: отказ не превращается в
// ошибку сессии.
//
// К моменту отказа ядро уже показало пользователю вопрос и перевело сессию в
// ожидание решения. Пришедший следом session.updateState("error") затёр бы это
// состояние, и вопрос исчез бы с экрана раньше, чем на него ответили, —
// подключиться стало бы невозможно вообще.
//
// Ошибка вносится через открытие канала: это единственный шов, через который
// тест дотягивается до run. Происхождение ошибки здесь роли не играет —
// проверяется реакция run на неё.
func TestRunDoesNotReportFailureWhenTheCoreIsAwaitingADecision(t *testing.T) {
	host := newFakeHost()
	channels := &fakeChannels{err: proxy.ErrPeerNotTrusted}
	m := NewManager(host, channels)

	if _, rpcErr := m.onConnect(connectRequest(t, validParams())); rpcErr != nil {
		t.Fatalf("onConnect: %v", rpcErr)
	}
	waitDone(t, m)

	for _, c := range host.callsTo("session.updateState") {
		p, ok := c.params.(updateStateParams)
		if ok && p.State == "error" {
			t.Fatalf("сессия объявлена ошибочной поверх заданного вопроса: %q", p.Error)
		}
	}
}

// А любая другая неудача обязана дойти до пользователя как раньше: молчаливый
// отказ оставил бы вкладку в connecting навсегда.
func TestRunStillReportsOrdinaryFailures(t *testing.T) {
	host := newFakeHost()
	m := NewManager(host, &fakeChannels{err: errors.New("канал не открылся")})

	if _, rpcErr := m.onConnect(connectRequest(t, validParams())); rpcErr != nil {
		t.Fatalf("onConnect: %v", rpcErr)
	}
	waitDone(t, m)

	if got := host.lastState(t); got.State != "error" {
		t.Fatalf("состояние %q, ожидалось error", got.State)
	}
}
