package proxy

import (
	"bytes"
	"encoding/binary"
	"testing"

	"xqs-plugin-rdp/internal/cleanpath"
	"xqs-plugin-rdp/internal/x224"
)

// csCorePDU собирает пакет, устроенный как настоящий MCS Connect Initial:
// префикс, блок CS_CORE с заявленной длиной и хвост.
//
// Размер блока и смещение поля — из MS-RDPBCGR 2.2.1.3.2, те же, по которым
// поле нашлось в живом пакете ironrdp-web (383 байта, блок на смещении 137).
func csCorePDU(prefixLen int, selected uint32) []byte {
	const blockLen = 216
	out := make([]byte, prefixLen)
	for i := range out {
		out[i] = 0xAA
	}
	block := make([]byte, blockLen)
	block[0], block[1] = csCoreTypeLo, csCoreTypeHi
	binary.LittleEndian.PutUint16(block[2:4], blockLen)
	binary.LittleEndian.PutUint32(block[csCoreSelectedOffset:csCoreSelectedOffset+4], selected)
	out = append(out, block...)
	return append(out, 0xBB, 0xBB)
}

// РЕГРЕССИЯ: серверу уходит протокол, который он сам и выбрал.
//
// Браузеру мы называем SSL — иначе клиент отвергнет ответ. Сервер по
// MS-RDPBCGR 3.3.5.3.1 обрывает соединение, если clientCoreData не подтверждает
// его выбор. Пока поле не правилось, это выглядело как бесконечное
// «authenticating»: рвётся уже после точки отсечения.
func TestPatchServerSelectedProtocolRewritesTheField(t *testing.T) {
	pdu := csCorePDU(137, x224.ProtocolSSL)

	if !patchServerSelectedProtocol(pdu, x224.ProtocolHybrid) {
		t.Fatal("блок CS_CORE не найден")
	}
	got := binary.LittleEndian.Uint32(pdu[137+csCoreSelectedOffset : 137+csCoreSelectedOffset+4])
	if got != x224.ProtocolHybrid {
		t.Fatalf("serverSelectedProtocol = %d, ожидался HYBRID", got)
	}
}

// Правится ровно четыре байта: всё остальное — данные клиента, и трогать их
// нам нечем и незачем.
func TestPatchServerSelectedProtocolTouchesNothingElse(t *testing.T) {
	pdu := csCorePDU(137, x224.ProtocolSSL)
	before := append([]byte(nil), pdu...)

	patchServerSelectedProtocol(pdu, x224.ProtocolHybrid)

	at := 137 + csCoreSelectedOffset
	if !bytes.Equal(before[:at], pdu[:at]) {
		t.Fatal("байты до поля изменились")
	}
	if !bytes.Equal(before[at+4:], pdu[at+4:]) {
		t.Fatal("байты после поля изменились")
	}
}

// Поле необязательное. Его отсутствие — не повод рвать подключение: серверу
// такому клиенту нечего предъявить.
func TestPatchServerSelectedProtocolReportsAMissingBlock(t *testing.T) {
	cases := map[string][]byte{
		"нет блока": bytes.Repeat([]byte{0xAA}, 300),
		"блок короче поля": func() []byte {
			b := make([]byte, 40)
			b[0], b[1] = csCoreTypeLo, csCoreTypeHi
			binary.LittleEndian.PutUint16(b[2:4], 40)
			return b
		}(),
		"длина за пределами пакета": func() []byte {
			b := make([]byte, 40)
			b[0], b[1] = csCoreTypeLo, csCoreTypeHi
			binary.LittleEndian.PutUint16(b[2:4], 300)
			return b
		}(),
	}
	for name, pdu := range cases {
		t.Run(name, func(t *testing.T) {
			if patchServerSelectedProtocol(pdu, x224.ProtocolHybrid) {
				t.Fatal("блок найден там, где его нет")
			}
		})
	}
}

// Читается ровно один пакет: сразу за ним идёт поток, который релей забирает
// себе, и лишний прочитанный байт увёл бы у него начало.
func TestForwardConnectInitialReadsExactlyOnePDU(t *testing.T) {
	pdu := csCorePDU(137, x224.ProtocolSSL)
	binary.BigEndian.PutUint16(pdu[2:4], uint16(len(pdu)))
	pdu[0] = 0x03
	trailing := []byte{0x11, 0x22, 0x33}

	browser := bytes.NewReader(append(append([]byte(nil), pdu...), trailing...))
	var server bytes.Buffer

	if err := forwardConnectInitial(browser, &server, x224.ProtocolHybrid); err != nil {
		t.Fatalf("forwardConnectInitial: %v", err)
	}
	if server.Len() != len(pdu) {
		t.Fatalf("серверу ушло %d байт, ожидалось %d", server.Len(), len(pdu))
	}
	sent := server.Bytes()
	got := binary.LittleEndian.Uint32(sent[137+csCoreSelectedOffset : 137+csCoreSelectedOffset+4])
	if got != x224.ProtocolHybrid {
		t.Fatalf("серверу ушёл протокол %d", got)
	}
	rest := make([]byte, 3)
	if _, err := browser.Read(rest); err != nil {
		t.Fatalf("хвост потока прочитан не полностью: %v", err)
	}
	if !bytes.Equal(rest, trailing) {
		t.Fatalf("из потока съедено лишнее: % x", rest)
	}
}

// Браузеру называется SSL — единственное, что примет клиент с выключенным
// CredSSP. Его собственные слова: «client advertised SSL, but server selected
// HYBRID» (ironrdp-connector/src/lib.rs:409), проверено на стенде.
func TestBrowserResponseTellsTheBrowserSSL(t *testing.T) {
	raw, err := browserResponse(serverLeg{
		chain:    [][]byte{{0x30, 0x82, 0x01}},
		selected: x224.ProtocolHybrid,
	}, "10.0.0.5:3389")
	if err != nil {
		t.Fatalf("browserResponse: %v", err)
	}
	got, err := cleanpath.ParseResponseForTest(raw)
	if err != nil {
		t.Fatalf("ParseResponseForTest: %v", err)
	}
	cc, err := x224.ParseConnectConfirm(got.X224)
	if err != nil {
		t.Fatalf("ParseConnectConfirm: %v", err)
	}
	if cc.Selected != x224.ProtocolSSL {
		t.Fatalf("браузеру названо %#x, а он примет только SSL", cc.Selected)
	}
	if len(got.CertChain) != 1 {
		t.Fatalf("цепочка сертификатов не доехала: %d", len(got.CertChain))
	}
}
