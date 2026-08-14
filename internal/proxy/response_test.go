package proxy

import (
	"bytes"
	"encoding/binary"
	"testing"

	"xqs-plugin-rdp/internal/cleanpath"
	"xqs-plugin-rdp/internal/x224"
)

func confirmPDU(selected uint32) []byte {
	out := []byte{0x03, 0x00, 0x00, 0x13, 0x0E, 0xD0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x08, 0x00}
	return binary.LittleEndian.AppendUint32(out, selected)
}

// РЕГРЕССИЯ: браузеру уходит Connect Confirm СЕРВЕРА, а не собранный нами.
//
// Клиент заполняет по нему clientCoreData.server_selected_protocol. Пока мы
// подставляли SSL, сервер, выбравший HYBRID, обрывал соединение на первом же
// MCS-сообщении — уже после точки отсечения, поэтому снаружи это выглядело как
// бесконечное «authenticating», а не как отказ подключения.
func TestBrowserResponseCarriesTheServersOwnConfirm(t *testing.T) {
	confirm := confirmPDU(x224.ProtocolHybrid)
	leg := serverLeg{
		chain:          [][]byte{{0x30, 0x82, 0x01}},
		connectConfirm: confirm,
	}

	raw, err := browserResponse(leg, "10.0.0.5:3389")
	if err != nil {
		t.Fatalf("browserResponse: %v", err)
	}
	got, err := cleanpath.ParseResponseForTest(raw)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if !bytes.Equal(got.X224, confirm) {
		t.Fatalf("браузеру ушёл не ответ сервера:\n got % x\nwant % x", got.X224, confirm)
	}

	cc, err := x224.ParseConnectConfirm(got.X224)
	if err != nil {
		t.Fatalf("ParseConnectConfirm: %v", err)
	}
	if cc.Selected != x224.ProtocolHybrid {
		t.Fatalf("Selected = %#x, ожидался HYBRID — иначе сервер оборвёт MCS", cc.Selected)
	}
}

// Цепочка сертификатов доезжает до браузера: без неё клиенту нечего показать
// пользователю о том, с кем он говорит.
func TestBrowserResponseCarriesTheCertificateChain(t *testing.T) {
	leg := serverLeg{
		chain:          [][]byte{{0x01, 0x02}, {0x03, 0x04}},
		connectConfirm: confirmPDU(x224.ProtocolHybrid),
	}

	raw, err := browserResponse(leg, "10.0.0.5:3389")
	if err != nil {
		t.Fatalf("browserResponse: %v", err)
	}
	got, err := cleanpath.ParseResponseForTest(raw)
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if len(got.CertChain) != 2 {
		t.Fatalf("сертификатов в ответе %d, ожидалось 2", len(got.CertChain))
	}
}
