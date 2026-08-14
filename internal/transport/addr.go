package transport

import "fmt"

// Addr — синтетический адрес канальной трубы.
//
// У канала нет ни IP, ни порта: другой конец строит и держит хост, а плагин в
// AppContainer вообще не имеет сетевых прав. Но net.Conn обязан отдавать адреса,
// и crypto/tls их спрашивает, поэтому нужен честный ответ вместо nil, на
// котором вызывающий получит панику.
//
// Сеть называется xqs-channel, а не подделывается под tcp: адрес, выглядящий
// как настоящий, ввёл бы в заблуждение и в логах, и в отладчике.
type Addr struct {
	Purpose   string
	ChannelID uint32
}

func (a Addr) Network() string { return "xqs-channel" }

func (a Addr) String() string {
	if a.Purpose == "" {
		return fmt.Sprintf("channel/%d", a.ChannelID)
	}
	return fmt.Sprintf("%s/%d", a.Purpose, a.ChannelID)
}
