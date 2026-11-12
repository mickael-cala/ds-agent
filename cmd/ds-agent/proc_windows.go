// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

//go:build windows

package main

import (
	"os"
	"os/exec"
)

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

// verrouExclusif : Windows n'a pas de flock POSIX. On n'empêche pas deux
// instances de partager une session (limitation connue, documentée dans ERRATA).
func verrouExclusif(f *os.File) error { return nil }

func libereVerrou(f *os.File) {}
