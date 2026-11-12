#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
# Copyright (C) 2025 les contributeurs de ds-agent

# ==========================================
# Agent de Build Go (Linux)
# ==========================================
set -euo pipefail

# ==========================================
# Configuration
# ==========================================
MODULE_NAME="github.com/mickael-cala/ds-agent"
MAIN_FILE="main.go"
OUTPUT_EXE="ds-agent"          # pas de .exe sous Linux

echo "=== Agent de Build Go (Linux) ==="
echo

# ==========================================
# 1. Vérification / Initialisation
# ==========================================
if [ ! -f "go.mod" ]; then
    echo "[INFO] Aucun fichier go.mod trouvé. Initialisation..."
    if ! go mod init "$MODULE_NAME"; then
        echo "[ERREUR] L'initialisation a échoué."
        exit 1
    fi
    echo "[SUCCES] Module initialisé."

    echo "[INFO] Nettoyage et téléchargement des dépendances..."
    go mod tidy
else
    echo "[INFO] Fichier go.mod détecté."
fi

# ==========================================
# 2. Compilation
# ==========================================
echo
echo "[INFO] Compilation en cours..."
if ! go build -ldflags="-s -w" -o "$OUTPUT_EXE" "$MAIN_FILE"; then
    echo "[ERREUR] La compilation a échoué."
    exit 1
fi
echo "[SUCCES] Compilation terminée : $OUTPUT_EXE"

# ==========================================
# 3. Exécution (Test)
# ==========================================
# Secrets : jamais en dur dans le dépôt — exportez-les dans votre shell.
: "${DEEPSEEK_API_KEY:?Définissez DEEPSEEK_API_KEY dans votre environnement avant de lancer ce script}"
export MCP_ROOT_SECRET="${MCP_ROOT_SECRET:-changeme}"
echo
echo "[INFO] Lancement de ./$OUTPUT_EXE..."
echo "--------------------------------------------------"

# On autorise l'échec du binaire sans tuer le script (set -e)
set +e
"./$OUTPUT_EXE"
EXIT_CODE=$?
set -e

echo "--------------------------------------------------"
echo "[INFO] Fin de l'exécution (Exit Code: $EXIT_CODE)."

exit $EXIT_CODE
