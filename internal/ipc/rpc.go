package ipc

import "encoding/json"

// Коды ошибок JSON-RPC 2.0.
const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternal       = -32603
)

const jsonRPCVersion = "2.0"

// Request — входящий вызов или нотификация от хоста.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification: запрос без id ответа не требует. Ответ на нотификацию —
// протокольная ошибка на нашей стороне, а не безобидная избыточность.
func (r Request) IsNotification() bool { return len(r.ID) == 0 }

// Error — объект ошибки JSON-RPC.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Response — ответ на запрос. Ровно одно из полей Result и Error попадает в
// вывод; какое именно, решает наличие Error.
type Response struct {
	// ID сохраняется байт в байт: JSON-RPC разрешает и число, и строку, и
	// подмена типа сломала бы сопоставление ответа с запросом у хоста.
	ID     json.RawMessage
	Result any
	Error  *Error
}

// MarshalJSON пишет ответ вручную, а не через теги структуры, из-за одного
// случая: `result: null` — законный успешный ответ (так отвечает, например,
// session.updateState), а `omitempty` выбросил бы ключ целиком и превратил
// успех в ответ без результата. Разделение на две формы делает это явным.
func (r Response) MarshalJSON() ([]byte, error) {
	id := r.ID
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	if r.Error != nil {
		return json.Marshal(struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   *Error          `json:"error"`
		}{jsonRPCVersion, id, r.Error})
	}
	return json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{jsonRPCVersion, id, r.Result})
}

func NewResult(id json.RawMessage, result any) Response {
	return Response{ID: id, Result: result}
}

func NewError(id json.RawMessage, code int, msg string) Response {
	return Response{ID: id, Error: &Error{Code: code, Message: msg}}
}
