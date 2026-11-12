// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// sandboxMode vaut "none" par défaut, "bwrap" avec --sandbox (ou DEEPSEEK_SANDBOX).
var sandboxMode = "none"

// shellCommand renvoie l'interpréteur et ses arguments pour exécuter `cmd`.
// Isolé du reste pour être testable sans lancer de processus.
func shellCommand(goos, cmd string) (string, []string) {
	if goos == "windows" {
		return "cmd", []string{"/c", cmd}
	}
	return "sh", []string{"-c", cmd}
}

// commandWithSandbox décide comment exécuter une commande shell.
//   - "none" : sh -c, comportement historique (aucun confinement) ;
//   - "bwrap" : bubblewrap, système de fichiers en lecture seule sauf le
//     répertoire de travail et un /tmp temporaire.
//
// `look` est injecté pour les tests (exec.LookPath en production).
// En l'absence de bwrap, on refuse d'exécuter : un faux bac à sable serait pire
// qu'une absence de bac à sable (l'utilisateur croirait être protégé).
func commandWithSandbox(mode, workDir, cmd string, look func(string) (string, error)) (string, []string, error) {
	sh, shArgs := shellCommand(runtime.GOOS, cmd)
	switch mode {
	case "", "none":
		return sh, shArgs, nil
	case "bwrap":
		if _, err := look("bwrap"); err != nil {
			return "", nil, fmt.Errorf("--sandbox exige bubblewrap (bwrap introuvable) : installez-le " +
				"(Termux : pkg install bubblewrap ; Debian : apt install bubblewrap) ou retirez --sandbox")
		}
		if strings.TrimSpace(workDir) == "" || !strings.HasPrefix(workDir, "/") {
			return "", nil, fmt.Errorf("sandbox : répertoire de travail absolu requis (%q)", workDir)
		}
		args := []string{
			"--die-with-parent",
			"--new-session",
			"--unshare-pid",
			"--unshare-uts",
			"--ro-bind", "/", "/",
			"--dev", "/dev",
			"--proc", "/proc",
			"--tmpfs", "/tmp",
			"--bind", workDir, workDir,
			"--chdir", workDir,
			"--clearenv",
			"--setenv", "HOME", workDir,
			"--setenv", "TMPDIR", "/tmp",
			"--setenv", "PATH", os.Getenv("PATH"),
			"--",
			sh, shArgs[0], shArgs[1],
		}
		return "bwrap", args, nil
	default:
		return "", nil, fmt.Errorf("mode sandbox inconnu : %q (attendu : none|bwrap)", mode)
	}
}

// isBwrapAvailable indique si le confinement est possible sur cette machine.
func isBwrapAvailable() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

// motifsDangereux : commandes qu'un agent ne doit pas lancer par accident.
// Le but n'est pas d'être exhaustif (impossible) mais d'arrêter les catastrophes
// irréversibles. /godmode assume explicitement le contournement.
var motifsDangereux = []struct {
	re    *regexp.Regexp
	motif string
}{
	{regexp.MustCompile(`(^|[;&|]\s*)(sudo\s+)?rm\s+(-[a-zA-Z]*[rf][a-zA-Z]*\s+)+/(\s|$)`), "rm -rf /"},
	{regexp.MustCompile(`\bmkfs(\.\w+)?\b`), "formatage de système de fichiers"},
	{regexp.MustCompile(`\bdd\b[^\n]*\bof=/dev/(sd|nvme|mmcblk|disk|hd)`), "écriture directe sur un disque"},
	{regexp.MustCompile(`>\s*/dev/(sd|nvme|mmcblk|disk|hd)`), "écrasement de périphérique"},
	{regexp.MustCompile(`(^|[;&|]\s*)chmod\s+-R\s+777\s+/(\s|$)`), "chmod -R 777 /"},
	{regexp.MustCompile(`(^|[;&|]\s*)(sudo\s+)?(shutdown|reboot|halt|poweroff)\b`), "arrêt du système"},
	{regexp.MustCompile(`:\s*\(\s*\)\s*\{`), "fork bomb"},
	{regexp.MustCompile(`\b(curl|wget)\b[^\n|]*\|\s*(ba)?sh\b`), "téléchargement exécuté directement"},
}

// motDetecte renvoie le motif destructeur trouvé dans une commande, ou "".
func motDetecte(c string) string {
	for _, m := range motifsDangereux {
		if m.re.MatchString(c) {
			return m.motif
		}
	}
	return ""
}
