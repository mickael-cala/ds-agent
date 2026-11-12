// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup isole la commande dans son propre groupe de processus : tuer
// le groupe emporte les petits-enfants (go test, npm, sleep…) et pas seulement
// le `sh -c` intermédiaire.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup tue la commande et tous ses descendants.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}

// verrouExclusif pose un verrou advisory non bloquant sur le fichier (flock).
// Il est libéré automatiquement à la mort du processus : pas de verrou périmé.
func verrouExclusif(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func libereVerrou(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
