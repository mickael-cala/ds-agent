// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

//go:build windows

package main

import "os/exec"

// setProcessGroup : Windows n'a pas de groupe de processus POSIX. On garde un
// no-op explicite plutôt qu'un comportement spécifique non testé.
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup tue au moins le processus direct (les petits-enfants cmd.exe
// peuvent survivre : limitation connue, documentée dans ERRATA).
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
