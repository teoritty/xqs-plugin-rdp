// Package session ведёт жизненный цикл одной embed-сессии: от session.connect
// до сноса, включая обращение с секретом.
//
// Он не знает ни про CredSSP, ни про формат RDP-сообщений — это забота
// internal/proxy. Здесь только порядок шагов и то, что происходит, когда
// какой-то из них не удался.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"xqs-plugin-rdp/internal/credssp"
	"xqs-plugin-rdp/internal/proxy"
)

// defaultPort совпадает с contributions.connectionProtocols[].defaultPort.
// Хост подставляет его сам, когда порт не задан, но полагаться на это без
// проверки нельзя: port = 0 в дозвоне выглядел бы как «любой порт».
const defaultPort = 3389

// connectParams — payload session.connect (docs/plugin-api.md).
type connectParams struct {
	SessionID    string            `json:"sessionId"`
	ConnectionID string            `json:"connectionId"`
	Protocol     string            `json:"protocol"`
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	Username     string            `json:"username"`
	Fields       map[string]string `json:"fields"`
}

// request — разобранный и проверенный запрос на подключение.
type request struct {
	sessionID string
	target    proxy.Target
}

var errNoSessionID = errors.New("session: в session.connect нет sessionId")

// parseConnect разбирает параметры и собирает цель подключения.
//
// Пароль извлекается как []byte и здесь же перестаёт существовать в виде
// строки настолько, насколько это возможно: строку из JSON занулить нельзя, и
// это ограничение стоит назвать вслух, а не делать вид, что его нет.
func parseConnect(raw json.RawMessage) (request, error) {
	var p connectParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return request{}, fmt.Errorf("session: разбор session.connect: %w", err)
	}
	if strings.TrimSpace(p.SessionID) == "" {
		return request{}, errNoSessionID
	}
	host := strings.TrimSpace(p.Host)
	if host == "" {
		return request{}, errors.New("session: в session.connect нет host")
	}
	port := p.Port
	if port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return request{}, fmt.Errorf("session: недопустимый порт %d", port)
	}

	username := strings.TrimSpace(p.Fields["username"])
	if username == "" {
		// Хост кладёт логин подключения в отдельное поле; наше поле формы
		// имеет приоритет, но пустым остаться не должно.
		username = strings.TrimSpace(p.Username)
	}
	if username == "" {
		return request{}, errors.New("session: не задано имя пользователя")
	}

	return request{
		sessionID: p.SessionID,
		target: proxy.Target{
			Address: net.JoinHostPort(host, strconv.Itoa(port)),
			// VerifyPeer подставляет жизненный цикл: там есть шина к ядру и
			// идентификатор сессии, а здесь — только разбор параметров.
			Credentials: credssp.Credentials{
				Domain:   strings.TrimSpace(p.Fields["domain"]),
				Username: username,
				Password: []byte(p.Fields["password"]),
			},
		},
	}, nil
}

// zeroCredentials зануляет пароль.
//
// Вызывается на КАЖДОМ пути выхода, включая ошибочные: секрет, переживший
// неудачное подключение, ничем не лучше секрета, пережившего удачное.
func zeroCredentials(c *credssp.Credentials) {
	for i := range c.Password {
		c.Password[i] = 0
	}
	c.Password = nil
}
