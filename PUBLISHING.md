# PUBLIER SUR GITHUB

Procédure complète pour transformer ce dépôt local en dépôt GitHub public, dans le bon
ordre (le renommage du module **doit** précéder le push, sinon `go install` et les badges
pointeront vers un chemin qui n'existe pas).

## 0. État actuel

- Dépôt git initialisé sur `main`, 8 commits, **aucun remote**, aucune authentification
  configurée sur cette machine (pas de `gh`, pas de `~/.git-credentials`, pas de clé privée SSH).
- Le module s'appelle encore `my_go_project` et le sous-projet `audit` (constat D01).
- Les commits sont signés `ds-agent <ds-agent@users.noreply.github.com>` (identité de
  travail, à remplacer par la vôtre : §3).

## 1. Créer le dépôt

Sur github.com : **New repository** → nom `ds-agent` → *Public* → **ne rien initialiser**
(pas de README, pas de licence : le dépôt local les a déjà).

L'URL sera `https://github.com/<compte>/ds-agent`.

## 2. Renommer le module

```bash
scripts/rename-module.sh <compte>          # compte GitHub, dépôt = ds-agent
# ou : scripts/rename-module.sh <compte> autre-nom
```

Le script réécrit `go.mod`, `audit_api/go.mod`, `build_and_run.sh/.cmd` et le badge du
README, puis vérifie `gofmt`, `go build`, `go vet`, `go test` (deux modules), `tests/e2e.sh`
et `tests/paranoid.sh`. Il ne commit rien : relisez `git diff` avant.

Mettre ensuite à jour la documentation du constat :

- `AUDIT.md` → D01 « ✅ corrigé (sprint 6) »
- `STATUS.md` → retirer la dette D8/D10 liée au nom de module
- `PLAN_ACTION.md` → sprint 6, cases cochées

## 3. Identité et auteurs

```bash
git config user.name  "Votre Nom"
git config user.email "vous@exemple.com"
```

Les commits existants portent l'identité de travail. Pour les réécrire **avant** le premier
push (aucun historique publié à ce stade, donc sans risque) :

```bash
git rebase --root --exec 'git commit --amend --no-edit --reset-author'
```

Vérification : `git log --format='%h %an <%ae>' | head`.

## 4. Authentification

Deux options, au choix.

**a) Jeton personnel (HTTPS) — le plus simple sous Termux**

1. GitHub → *Settings → Developer settings → Personal access tokens → Fine-grained* :
   accès limité au dépôt `ds-agent`, permission **Contents: Read and write**.
2. Stocker le jeton (une fois, hors du dépôt) :

```bash
git config --global credential.helper store   # écrit ~/.git-credentials en 0600
# puis au premier push, login = <compte>, mot de passe = le jeton
```

**b) Clé SSH**

```bash
ssh-keygen -t ed25519 -C "ds-agent"          # ~/.ssh/id_ed25519
cat ~/.ssh/id_ed25519.pub                    # à coller dans GitHub → SSH and GPG keys
ssh -T git@github.com                        # doit répondre « successfully authenticated »
git remote add origin git@github.com:<compte>/ds-agent.git
```

> Le fichier `~/.ssh/authorized_keys` présent sur cette machine est une clé **entrante**
> (pour se connecter à Termux), pas une clé pour GitHub.

## 5. Contrôles avant push

```bash
# aucun secret versionné
git grep -nE 'sk-[A-Za-z0-9]{16,}|ghp_[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}' $(git rev-parse HEAD) && echo "SECRET !" || echo "aucun secret"

# aucun fichier interdit suivi
git ls-files | grep -E '(^|/)(\.env$|mcp\.json$|.*\.(exe|bak|bak[0-9]+)$)' && echo "fichier interdit suivi" || echo "arborescence propre"

# tout passe
gofmt -l . ; go vet ./... ; go test -count=1 ./... ; (cd audit_api && go test -count=1 ./...)
bash tests/paranoid.sh ; bash tests/e2e.sh
```

Rappel : la clé DeepSeek qui a séjourné dans `build_and_run.sh` **doit être révoquée** sur
platform.deepseek.com — elle a existé en clair sur ce disque.

## 6. Publier

```bash
git remote add origin https://github.com/<compte>/ds-agent.git   # ou git@github.com:…
git branch -M main
git push -u origin main
```

## 7. Après le push

1. **CI** : onglet *Actions* → les 3 jobs (`qualite`, `build` ×3 OS, `vulnerabilites`)
   doivent être verts. Si `setup-go` échoue, c'est que la version déclarée dans `go.mod`
   (`1.27.1`) n'existe pas côté runner : ajuster la directive `go` ou l'épingler.
2. **Badge** : `README.md` contient l'URL du badge CI ; le script de renommage l'a alignée.
3. **Réglages recommandés** : *Settings → Branches* → protection de `main` (PR + CI verte),
   *Security → Dependabot alerts* + *Secret scanning*, description + topics
   (`go`, `cli`, `agent`, `deepseek`, `mcp`).
4. **Licence** : le dépôt est double AGPL/commercial ; GitHub affichera « View license » à
   partir de `LICENSE` (AGPL) — la licence commerciale est décrite dans `LICENSE-COMMERCIAL.md`.
5. **Première release** : `git tag -a v0.2.0 -m "…" && git push --tags`, puis *Releases* →
   joindre les binaires (`goreleaser` prévu en ROADMAP v0.2).

## 8. Si le compte change d'avis plus tard

Le renommage est réversible : relancer `scripts/rename-module.sh <nouveau-compte>` puis
`git remote set-url origin …`, ou renommer le dépôt GitHub (*Settings → Rename*), qui
redirige automatiquement l'ancienne URL.
