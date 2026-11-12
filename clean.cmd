@echo off
setlocal

REM ==========================================
REM Configuration
REM ==========================================
set EXE_NAME=ds-agent.exe
set STATE_DIR=%APPDATA%\ds-agent
set SESSIONS_DIR=%STATE_DIR%\sessions
set AUDIT_FILE=%STATE_DIR%\audit.jsonl

echo === Nettoyage du projet Go ===
echo/

REM 1. Suppression de l'executable
if exist "%EXE_NAME%" (
    echo [INFO] Suppression de %EXE_NAME%...
    del /f /q "%EXE_NAME%"
    if exist "%EXE_NAME%" (
        echo [ERREUR] Impossible de supprimer %EXE_NAME%. Est-il en cours d'execution ?
    ) else (
        echo [SUCCES] Executable supprime.
    )
) else (
    echo [INFO] Aucun executable %EXE_NAME% trouve.
)

REM 2. Nettoyage (Optionnel) des fichiers de configuration Go
REM if exist "go.mod" del /f /q "go.mod"
REM if exist "go.sum" del /f /q "go.sum"

REM 3. Nettoyage du journal d'audit
if exist "%AUDIT_FILE%" (
    echo [INFO] Suppression du journal d'audit ^(%AUDIT_FILE%^)...
    del /f /q "%AUDIT_FILE%"
)

REM 4. Nettoyage du cache de session local de l'agent
if exist "%SESSIONS_DIR%" (
    echo [INFO] Suppression du cache des sessions ^(%SESSIONS_DIR%^)...
    rmdir /s /q "%SESSIONS_DIR%"
    echo [SUCCES] Cache de session nettoye.
) else (
    echo [INFO] Aucun cache de session trouve.
)

echo/
echo [SUCCES] Projet nettoye.

endlocal
