// Command xqs-rdp — RDP-плагин xQuakShell.
//
// Это композиционный корень: он только связывает пакеты и не содержит логики.
// Всё, что умеет думать, живёт в internal/.
package main

import (
	"errors"
	"log/slog"
	"os"

	"xqs-plugin-rdp/internal/ipc"
	"xqs-plugin-rdp/internal/lifecycle"
)

func main() {
	// stdout занят канальной шиной — любой байт мимо фрейма ломает протокол и
	// стоит нам процесса. Логи идут только в stderr.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	if code := run(); code != 0 {
		os.Exit(code)
	}
}

func run() int {
	manager := lifecycle.NewManager()
	dispatcher := ipc.NewDispatcher(os.Stdout)
	manager.Register(dispatcher)

	served := make(chan error, 1)
	go func() { served <- dispatcher.Serve(os.Stdin) }()

	select {
	case <-manager.Done():
		// Штатное завершение по shutdown от ядра.
		slog.Info("xqs-rdp: shutdown requested, exiting")
		return 0
	case err := <-served:
		return exitCodeFor(err)
	}
}

// exitCodeFor различает два конца потока, которые легко перепутать: ядро
// закрыло stdio (это конец работы) и мы нарушили протокол (это дефект, и он
// обязан быть виден в коде выхода, а не утонуть в логе).
func exitCodeFor(err error) int {
	switch {
	case err == nil:
		slog.Info("xqs-rdp: host closed stdio, exiting")
		return 0
	case errors.Is(err, ipc.ErrProtocol):
		slog.Error("xqs-rdp: protocol violation on the bus", "err", err.Error())
		return 2
	default:
		slog.Error("xqs-rdp: bus read failed", "err", err.Error())
		return 1
	}
}

// declaredPluginAPI — версия Plugin API, о которой сообщает код. Тест сверяет её
// с requires.pluginApi в манифесте.
func declaredPluginAPI() string { return lifecycle.APIVersion() }
