package session

import (
	"context"
	"encoding/base64"
	"fmt"
)

// Обращение к ядру за доверием к идентичности сервера.
//
// Отдельным файлом, а не строкой в lifecycle.go: здесь проходит граница
// доверия, и её стоит читать целиком, а не искать между регистрацией embed и
// раздачей кредитов.

// trustTimeout — срок вызова trust.verifyPeer.
//
// Он не включает ожидание пользователя. Ядро отвечает сразу: «доверяем» или
// «нет, вопрос задан», а решение человека приезжает потом отдельным
// подключением. Поэтому здесь обычный короткий срок RPC, а не минуты.
const trustTimeout = rpcTimeout

type verifyPeerParams struct {
	SessionID string `json:"sessionId"`
	// Subject — наше представление о том, с кем мы говорим. Ядро сверяет его с
	// записью соединения и дальше пользуется своей версией. Расхождение —
	// отказ: если мы считаем, что говорим с одним, а отправлены к другому, это
	// либо дефект, либо атака.
	Subject string `json:"subject"`
	// MaterialBase64 — наблюдённые байты. Отпечаток считает ядро: прислать
	// готовый значило бы дать возможность показать одно, а сохранить другое.
	MaterialBase64 string `json:"materialBase64"`
}

type verifyPeerResult struct {
	Trusted bool `json:"trusted"`
}

// verifyPeer спрашивает ядро про сервер этой сессии.
//
// Возврат false — не ошибка: ядро в этот момент уже показало пользователю
// вопрос и перевело сессию в ожидание решения. Ошибкой это станет выше, и
// именно такой, которую вызывающий обязан отличить от сбоя.
func (m *Manager) verifyPeer(parent context.Context, sessionID, subject string, material []byte) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, trustTimeout)
	defer cancel()

	var res verifyPeerResult
	err := m.host.Call(ctx, "trust.verifyPeer", verifyPeerParams{
		SessionID:      sessionID,
		Subject:        subject,
		MaterialBase64: base64.StdEncoding.EncodeToString(material),
	}, &res)
	if err != nil {
		return false, fmt.Errorf("session: trust.verifyPeer: %w", err)
	}
	return res.Trusted, nil
}
