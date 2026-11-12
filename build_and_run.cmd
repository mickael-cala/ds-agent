@echo off
@REM SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
@REM Copyright (C) 2025 les contributeurs de ds-agent

setlocal

REM ==========================================
REM Configuration
REM ==========================================
set MODULE_NAME=my_go_project
set MAIN_FILE=main.go
set OUTPUT_EXE=ds-agent.exe

echo === Agent de Build Go (Windows) ===
echo/

REM ==========================================
REM 1. Verification / Initialisation
REM ==========================================
if not exist "go.mod" (
    echo [INFO] Aucun fichier go.mod trouve. Initialisation...
    go mod init %MODULE_NAME%
    if errorlevel 1 (
        echo [ERREUR] L'initialisation a echoue.
        goto :end
    )
    echo [SUCCES] Module initialise.

    echo [INFO] Nettoyage et telechargement des dependances...
    go mod tidy
) else (
    echo [INFO] Fichier go.mod detecte.
)

REM ==========================================
REM 2. Compilation
REM ==========================================
echo/
echo [INFO] Compilation en cours...
go build -ldflags="-s -w" -o %OUTPUT_EXE% %MAIN_FILE%

if %ERRORLEVEL% NEQ 0 (
    echo [ERREUR] La compilation a echoue ^(Exit Code: %ERRORLEVEL%^).
    goto :end
)
echo [SUCCES] Compilation terminee : %OUTPUT_EXE%

REM ==========================================
REM 3. Execution (Test)
REM ==========================================
REM Secrets : jamais en dur dans le depot. Definissez-les avant de lancer ce script.
if "%DEEPSEEK_API_KEY%"=="" (
    echo [ERREUR] Definissez DEEPSEEK_API_KEY dans votre environnement.
    goto :end
)
if "%MCP_ROOT_SECRET%"=="" set MCP_ROOT_SECRET=changeme
echo/
echo [INFO] Lancement de %OUTPUT_EXE%...
echo --------------------------------------------------
%OUTPUT_EXE%
echo --------------------------------------------------
echo [INFO] Fin de l'execution (Exit Code: %ERRORLEVEL%).

:end