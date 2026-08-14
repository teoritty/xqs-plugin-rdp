package credssp

import (
	"errors"
	"fmt"
	"io"

	"github.com/bodgit/ntlmssp"
)

// maxTSRequest — потолок одного TSRequest.
//
// Длина приходит от сервера, с которым мы ещё не аутентифицировались, то есть
// от недоверенной стороны. Без потолка первый же встречный смог бы заказать
// нам аллокацию любого размера.
const maxTSRequest = 64 << 10

// Коды NTSTATUS, которые сервер реально возвращает на этом шаге.
const (
	statusLogonFailure        uint32 = 0xC000006D
	statusAccountRestriction  uint32 = 0xC000006E
	statusInvalidLogonHours   uint32 = 0xC000006F
	statusInvalidWorkstation  uint32 = 0xC0000070
	statusPasswordExpired     uint32 = 0xC0000071
	statusAccountDisabled     uint32 = 0xC0000072
	statusAccountLockedOut    uint32 = 0xC0000234
	statusPasswordMustChange  uint32 = 0xC0000224
	statusLogonTypeNotGranted uint32 = 0xC000015B
)

// ErrRejected — сервер вернул errorCode.
type ErrRejected struct {
	// Code — NTSTATUS. Хранится как uint32, а не int: в DER он приходит знаковым,
	// и 0xC000006D читается как -0x3fffff93 — значение, по которому невозможно
	// ни узнать статус, ни найти его в документации.
	Code uint32
}

func (e *ErrRejected) Error() string {
	return fmt.Sprintf("credssp: сервер отклонил аутентификацию: %s (NTSTATUS %#08x)", ntstatusText(e.Code), e.Code)
}

// ntstatusText переводит статус в то, что пользователь может починить.
// «Неверный логин или пароль» и «учётная запись заблокирована» требуют разных
// действий, и различить их — единственная причина, по которой эта таблица есть.
func ntstatusText(code uint32) string {
	switch code {
	case statusLogonFailure:
		return "неверный логин или пароль"
	case statusAccountRestriction:
		return "учётной записи запрещён этот вход"
	case statusInvalidLogonHours:
		return "вход запрещён в это время суток"
	case statusInvalidWorkstation:
		return "вход с этой рабочей станции запрещён"
	case statusPasswordExpired:
		return "срок действия пароля истёк"
	case statusAccountDisabled:
		return "учётная запись отключена"
	case statusAccountLockedOut:
		return "учётная запись заблокирована"
	case statusPasswordMustChange:
		return "пароль требуется сменить"
	case statusLogonTypeNotGranted:
		return "у учётной записи нет права на удалённый вход"
	default:
		return "причина неизвестна"
	}
}

// Credentials — то, чем аутентифицируемся.
//
// Password — []byte, а не string, и владеет им вызывающий: пакет его не
// зануляет, потому что не знает, нужен ли пароль дальше (например, для
// повторной попытки).
type Credentials struct {
	Domain   string
	Username string
	Password []byte
}

// Authenticate проводит полный обмен CredSSP поверх уже установленного TLS.
//
// rw — открытый поток внутри TLS-туннеля к серверу. serverPubKey — DER
// SubjectPublicKeyInfo сертификата сервера, тот же материал, по которому
// internal/trust считает отпечаток.
//
// Порядок обмена (MS-CSSP 3.1.5):
//
//	1..3. NTLM negotiate/challenge/authenticate внутри negoTokens
//	4.    клиент шлёт clientNonce + pubKeyAuth = Wrap(ClientServerHash)
//	5.    сервер отвечает pubKeyAuth = Wrap(ServerClientHash) — проверяем
//	6.    клиент шлёт authInfo = Wrap(TSCredentials)
//
// Шаг 5 — не формальность: он доказывает, что тот, кто владеет ключом сессии
// NTLM, владеет и ключом сертификата. Пропустить его значит согласиться, что
// между нами и сервером мог стоять кто угодно.
func Authenticate(rw io.ReadWriter, serverPubKey []byte, creds Credentials) error {
	if len(serverPubKey) == 0 {
		return errors.New("credssp: пустой открытый ключ сервера")
	}

	client, err := ntlmssp.NewClient(
		ntlmssp.SetUserInfo(creds.Username, string(creds.Password)),
		ntlmssp.SetDomain(creds.Domain),
	)
	if err != nil {
		return fmt.Errorf("credssp: инициализация NTLM: %w", err)
	}

	// Шаг 1: NTLM Negotiate.
	token, err := client.Authenticate(nil, nil)
	if err != nil {
		return fmt.Errorf("credssp: NTLM negotiate: %w", err)
	}

	var serverVersion int
	for {
		if err := writeTSRequest(rw, TSRequest{Version: Version, NegoToken: token}); err != nil {
			return err
		}
		resp, err := readTSRequest(rw)
		if err != nil {
			return err
		}
		if resp.HasError {
			return &ErrRejected{Code: uint32(resp.ErrorCode)}
		}
		// Версию сервер сообщает в первом же ответе; проверяем сразу, чтобы не
		// доводить обмен до конца с непропатченным сервером.
		if serverVersion == 0 {
			serverVersion = resp.Version
			if err := CheckServerVersion(serverVersion); err != nil {
				return err
			}
		}
		if client.Complete() {
			break
		}
		if len(resp.NegoToken) == 0 {
			return errors.New("credssp: сервер не прислал NTLM-токен, а обмен не завершён")
		}
		token, err = client.Authenticate(resp.NegoToken, nil)
		if err != nil {
			return fmt.Errorf("credssp: NTLM: %w", err)
		}
		if len(token) == 0 {
			return errors.New("credssp: NTLM не дал токена, а обмен не завершён")
		}
	}

	session := client.SecuritySession()
	if session == nil {
		return errors.New("credssp: NTLM завершён без сессионной безопасности — нечем шифровать учётные данные")
	}

	// Шаг 4: привязка к открытому ключу сервера.
	nonce, err := NewNonce()
	if err != nil {
		return err
	}
	sealedHash, err := wrap(session, ClientServerHash(nonce, serverPubKey))
	if err != nil {
		return fmt.Errorf("credssp: упаковка pubKeyAuth: %w", err)
	}
	if err := writeTSRequest(rw, TSRequest{
		Version:     Version,
		PubKeyAuth:  sealedHash,
		ClientNonce: nonce,
	}); err != nil {
		return err
	}

	// Шаг 5: сервер обязан доказать владение ключом.
	resp, err := readTSRequest(rw)
	if err != nil {
		return err
	}
	if resp.HasError {
		return &ErrRejected{Code: uint32(resp.ErrorCode)}
	}
	if len(resp.PubKeyAuth) == 0 {
		return errors.New("credssp: сервер не подтвердил привязку к открытому ключу")
	}
	serverHash, err := unwrap(session, resp.PubKeyAuth)
	if err != nil {
		return fmt.Errorf("credssp: распаковка ответа сервера: %w", err)
	}
	if err := VerifyServerHash(serverHash, nonce, serverPubKey); err != nil {
		return err
	}

	// Шаг 6: учётные данные. Открытая форма живёт ровно до шифрования.
	plain, err := EncodeCredentials(creds.Domain, creds.Username, creds.Password)
	if err != nil {
		return err
	}
	defer clearBytes(plain)
	sealedCreds, err := wrap(session, plain)
	if err != nil {
		return fmt.Errorf("credssp: упаковка учётных данных: %w", err)
	}
	return writeTSRequest(rw, TSRequest{Version: Version, AuthInfo: sealedCreds})
}

// wrap шифрует и подписывает по правилам GSS-обёртки NTLM.
//
// MS-CSSP ждёт подпись ПЕРЕД зашифрованными данными, а Wrap возвращает их
// раздельно. Порядок здесь — не деталь оформления: перепутанный, он даёт
// сообщение, которое сервер отвергнет без объяснений.
func wrap(s *ntlmssp.SecuritySession, plaintext []byte) ([]byte, error) {
	sealed, signature, err := s.Wrap(plaintext)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(signature)+len(sealed))
	out = append(out, signature...)
	out = append(out, sealed...)
	return out, nil
}

// ntlmSignatureLen — длина подписи NTLM (MS-NLMP 2.2.2.9, версия 1).
const ntlmSignatureLen = 16

func unwrap(s *ntlmssp.SecuritySession, wrapped []byte) ([]byte, error) {
	if len(wrapped) < ntlmSignatureLen {
		return nil, fmt.Errorf("credssp: сообщение %d байт короче подписи NTLM", len(wrapped))
	}
	return s.Unwrap(wrapped[ntlmSignatureLen:], wrapped[:ntlmSignatureLen])
}

func writeTSRequest(w io.Writer, r TSRequest) error {
	raw, err := r.Marshal()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("credssp: отправка TSRequest: %w", err)
	}
	return nil
}

// readTSRequest читает ровно один TSRequest.
//
// TSRequest — DER SEQUENCE, и его длина известна из заголовка, поэтому читаем
// заголовок, затем ровно тело. Читать «сколько дадут» здесь нельзя: за
// TSRequest в том же потоке идут следующие сообщения.
func readTSRequest(r io.Reader) (TSRequest, error) {
	header, bodyLen, err := readDERHeader(r)
	if err != nil {
		return TSRequest{}, err
	}
	if bodyLen > maxTSRequest {
		return TSRequest{}, fmt.Errorf("credssp: TSRequest %d байт сверх потолка %d", bodyLen, maxTSRequest)
	}
	buf := make([]byte, len(header)+bodyLen)
	copy(buf, header)
	if _, err := io.ReadFull(r, buf[len(header):]); err != nil {
		return TSRequest{}, fmt.Errorf("credssp: чтение TSRequest: %w", err)
	}
	return UnmarshalTSRequest(buf)
}

// readDERHeader читает тег и длину DER, возвращая прочитанные байты и длину
// тела.
func readDERHeader(r io.Reader) ([]byte, int, error) {
	var head [2]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, 0, fmt.Errorf("credssp: чтение заголовка TSRequest: %w", err)
	}
	if head[0] != 0x30 {
		return nil, 0, fmt.Errorf("credssp: ожидался DER SEQUENCE, получен тег %#x", head[0])
	}
	header := []byte{head[0], head[1]}

	if head[1] < 0x80 {
		return header, int(head[1]), nil
	}
	n := int(head[1] & 0x7F)
	if n == 0 || n > 4 {
		return nil, 0, fmt.Errorf("credssp: неподдерживаемая форма длины DER (%d байт)", n)
	}
	lenBytes := make([]byte, n)
	if _, err := io.ReadFull(r, lenBytes); err != nil {
		return nil, 0, fmt.Errorf("credssp: чтение длины TSRequest: %w", err)
	}
	header = append(header, lenBytes...)
	length := 0
	for _, b := range lenBytes {
		length = length<<8 | int(b)
	}
	if length < 0 {
		return nil, 0, errors.New("credssp: отрицательная длина TSRequest")
	}
	return header, length, nil
}
