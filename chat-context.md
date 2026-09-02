# chat-context.md — Guide de contexte pour agents IA (sandboxd)

Ce fichier est conçu pour qu'une IA (ou un humain) puisse **reprendre le travail
sur ce projet sans repartir de zéro**. Il explique l'état actuel du fork, les
modifications déjà apportées, les règles à respecter quand on modifie le code,
le versioning, les commits et le déploiement (staging + prod). À LIRE INTÉGRALEMENT
avant toute modification de code.

---

## 1. Qu'est-ce que ce projet ?

**sandboxd** est un service de conteneurs de dev isolés ("sandboxes"), avec une
URL de preview HTTP/HTTPS pour chaque sandbox. Il tourne entièrement sur Docker :
un control plane Go (`sandboxd`), un reverse-proxy Traefik, et chaque sandbox
est un conteneur Docker frère. Les sandboxes s'endorment à l'inactivité et se
réveillent à la demande. Les workspaces persistent sur disque.

Ce dépôt est un **fork** de https://github.com/tastyeffectco/sandboxd.git
(le dépôt officiel sandboxd.io), maintenu pour des besoins personnels.

**Remotes git :**
- `origin` → https://github.com/tastyeffectco/sandboxd.git (upstream officiel, lecture seule)
- `fork`  → https://github.com/nordineonline-sudo/sandboxd.git (LE dépôt de travail, push `main` ici)

> ⚠️ Ne poussez JAMAIS vers `origin` (upstream). Tout se pousse vers `fork`.

---

## 2. État actuel (au dernier commit sur `fork/main`)

Destinations concrètes :
- **Console prod** : http://console.192.168.10.2.nip.io  (API : `127.0.0.1:9090`)
- **Console staging** : http://console-staging.192.168.10.2.nip.io:9001  (API : `127.0.0.1:9002`)
- Preview d'un sandbox : `http://s-<id>-<port>.preview.<PREVIEW_DOMAIN>:<HTTP_PORT>`

État des versions :
- **Prod** : images `sandboxd-control-plane` / `sandboxd-console` : `0.4.1-nordineonline.3`
  (l'upgrade vers `.7` n'a PAS encore été déployé en prod — à faire sur validation).
- **Staging** : images `.7` déployées (retaguées sur le tag `0.4.1-nordineonline.3`
  que le compose staging référence, via `docker tag`).

### Version affichée dans Settings
`GET /v1/settings` → `version` renvoie la valeur stampée au build du control
plane. Elle est injectée par l'argument `VERSION` (default `dev`). Pour qu'une
version réelle apparaisse, il faut builder avec :
`SANDBOXD_VERSION=0.4.1-nordineonline.<n> SANDBOXD_GIT_COMMIT=$(git rev-parse --short HEAD) docker compose build sandboxd`

---

## 3. Historique des versions du fork (ce qui a déjà été fait)

Le fork publie sous `0.4.1-nordineonline.<n>` (les tags `v*` d'upstream ne sont
pas utilisés). Historique :

| Version | Contenu |
|---|---|
| `0.4.1-nordineonline.1` | Onglet agent dédié (pleine largeur) + onglet README. |
| `0.4.1-nordineonline.2` | **Gestionnaire de fichiers** complet dans l'onglet Files (upload multi + drag-drop dossiers, download fichier/dossier en zip, mkdir/rename/delete, preview images). |
| `0.4.1-nordineonline.3` | **Instructions agent personnalisées** (Settings → "Agent instructions (custom)") — prompt système global additif, persisté, appliqué aux prochaines tâches. |
| `0.4.1-nordineonline.4` | **Refonte navigation + chat** : sidebar fixe desktop / drawer hamburger mobile ; nouveau **chat headless** façon Telegram/WhatsApp (bulles, textarea auto-expansible, Entrée sans envoi involontaire sur mobile) branché sur l'API tasks/SSE ; l'iframe OpenCode natif devient un onglet ; **6 providers** câblés en plus de MiniMax (OpenAI, DeepSeek, OpenRouter, Cerebras, NVIDIA, xAI). |
| `0.4.1-nordineonline.5` | **Catalogue de modèles dynamique** : `GET /v1/agents/{id}/models` fait un appel `GET <base>/models` (OpenAI-compatible) côté control plane ; dropdown de modèles dans le chat. |
| `0.4.1-nordineonline.6` | **Séparation provider/modèle + correction d'un bug de routage** : un modèle `openrouter/…` retombait silencieusement sur Zen. Cause racine : la logique vit dans **`runtimed`, compilé dans l'image SANDBOX `sandboxd-base`**, pas dans le control plane → il fallait reconstruire `sandboxd-base` ET recréer les conteneurs sandbox (voir §6). |
| `0.4.1-nordineonline.7` | **6 providers de + câblés** (Mistral, Vercel AI Gateway, Hugging Face, Z.AI, Google/Gemini via en-tête `x-goog-api-key`, Perplexity sans catalogue public) ; suppression du sélecteur d'agent redondant (toujours `opencode`) ; liste des providers affiche tous les providers câblés, connectés ou non ; **bouton Stop** (annulation via `POST …/tasks/{id}/cancel`) ; mémorisation du choix provider/modèle par sandbox (`localStorage`) ; onglet « Advanced » renommé **« OpenCode »** et activé sur mobile (seul endroit avec les vrais QCM/permissions interactifs d'OpenCode, car le chat headless lance `opencode run --dangerously-skip-permissions`). |
| `0.4.1-nordineonline.8` | Suppression du **badge flottant « ? »** ("Ask sandboxd / Platform-wide help", composant `Helper` dans `console/src/App.tsx`) ; ajout de ce fichier `chat-context.md`. |

### Structure cœur à connaître
- `control-plane/` : le backend Go (API `/v1/*`, proxy credentials, orchestrateur Docker).
- `control-plane/cmd/runtimed/` : le superviseur DANS le sandbox (lance le serveur web,
  exécute les tâches agent, héberge `opencode web`). **Fichiers clés** :
  `opencode.go` (routage providers via `opencodeUpstream`/`writeOpencodeProxyConfig`),
  `agentenv.go` (scrubbing des secrets + clés factices), `opencodeweb.go`.
- `control-plane/internal/authproxy/proxy.go` : le **proxy d'injection de credentials**
  (les clés ne pénètrent jamais dans le sandbox ; le proxy les injecte sur le réseau).
- `control-plane/internal/agentauth/` : registre des providers + stockage des clés.
- `control-plane/internal/api/` : routes HTTP ; `v1_agent_models.go` (catalogue de modèles).
- `console/src/` : l'app React (`App.tsx`, `AppView.tsx`, `SettingsView.tsx`, `Sidebar.tsx`).
- `image/` : le Dockerfile de l'image SANDBOX (`sandboxd-base`).
- `docs/openapi.yaml` : contrat API (testé : `TestOpenAPIContractMatchesRoutes`).

---

## 4. Règles absolues quand on modifie le code

1. **Ne jamais modifier le binaire OpenCode lui-même** (paquet npm `opencode-ai`).
   Il est téléchargé à l'installation dans `image/Dockerfile` ; toute modif serait
   "one-shot" perdue. On n'en change PAS le comportement.
2. **`cmd/runtimed/opencode.go`, `cmd/runtimed/agentenv.go`, `cmd/runtimed/opencodeweb.go`,
   `internal/authproxy/`, `internal/agentauth/`, `image/Dockerfile` NE doivent être
   modifiés qu'avec une validation explicite de l'utilisateur** — ce sont des fichiers
   "moteur". Ce sont du code sandboxd (pas le binaire OpenCode), donc modifiables,
   mais uniquement sur demande validée.
3. **Adopter le comportement de l'utilisateur pendant le chat** :
   - reformuler / corriger les demandes ambiguës ;
   - en cas de doute ou d'ambiguïté, poser une **QCM interactive** (outil `question`)
     plutôt que de deviner ;
   - établir un **plan** et le présenter AVANT d'écrire du code ; ne commencer à
     coder qu'après confirmation explicite (« GO » / « je valide »).
4. **Ne jamais casser ce qui marche déjà** : le chat headless, le catalogue de
   modèles, le file manager, les custom agent instructions, la navigation sidebar.
5. Tests : `go build/vet/test` doivent rester verts côté Go ; `tsc`/`vite build`
   et `vitest run` verts côté console (14 tests dans `console/src/brain.test.ts`).
6. **Piège de déploiement connu** : toute modif de `cmd/runtimed/**.go` (routage,
   env, exec de tâches) n'est effective qu'après (a) `docker build -f image/Dockerfile`
   de `sandboxd-base`, et (b) **re-création des conteneurs sandbox** (voir §7-3).
   Une simple recréation du control plane / un stop-start de sandbox n'applique
   PAS le nouveau `runtimed`.

---

## 5. Comment apporter une nouvelle modification (workflow)

1. **Analyse** : lire `chat-context.md` + les fichiers concernés.
2. **Plan** : présenter un plan détaillé (fichiers touchés, fonctions conservées,
   ajouts, impact). Ne pas commencer avant un « GO » explicite.
3. **Implémentation** :
   - Côté Go : suivre les patterns existants (tests dans chaque package).
   - Côté console : pattern inline-style + `design/kit.tsx` (pas de Tailwind).
4. **Validation** :
   - `control-plane/` : `go build ./... && go vet ./... && go test ./...`
     (via le conteneur `golang:1.22-bookworm`, avec les caches montés — voir §8).
   - `console/` : `npx tsc --noEmit -p .` (une erreur PRÉEXISTANTE sur `Card` +
     `onDragOver` ligne ~1272 de `AppView.tsx` est connue et NON liée à nos modifs) ;
     `npm run build` ; `npx vitest run`.
   - Déployer en staging et valider côté réel (Playwright ou à la main).
5. **Bump de version** (voir §6) + mise à jour des docs (README/CHANGELOG).
6. **Commit + push** vers `fork/main` (voir §7-1).

---

## 6. Versioning

- Le fork publie `0.4.1-nordineonline.<n>` : chaque release incrémente `<n>`.
- **Fichiers à bumper ensemble** (pattern utilisé à chaque release) :
  - `console/package.json` (`"version"`)
  - `docker-compose.yml` (images `sandboxd-control-plane`, `sandboxd-console`,
    env `SANDBOXD_IMAGE` du `sandboxd-base`) — attention : le sed global
    `0.4.1-nordineonline.<n-1>` → `<n>` touche AUSSI l'onglet historique du README ;
    le vérifier/corriger ensuite.
  - `README.md` : badge d'en-tête, section 8 (« Où le fork diffère »), et le
    **tableau de suivi des versions** en bas.
  - `CHANGELOG.md` : nouvelle entrée en tête avec la date (`YYYY-MM-DD`).
  - `image/README.md` : chemins de build de l'image base.
- Quand on reconstruit pour déployer, la version affichée dans Settings doit être
  stampée : `SANDBOXD_VERSION=0.4.1-nordineonline.<n> SANDBOXD_GIT_COMMIT=$(git rev-parse --short HEAD)`.

---

## 7. Commit, push et déploiement

### 7-1. Commit + push
- Toujours sur `main` (locale = `main`). Commit message :
  `release(0.4.1-nordineonline.<n>): <résumé>`
- Push : `git push fork main` (jamais `origin`).
- Pré-série : `git fetch fork main` pour vérifier que `fork/main` n'a pas divergé.
  ⚠️ Attention : il y a eu un historique corrompu (un commit d'un autre
  projet/agent `e9ba000 "0.4.2-nordineonline.1"` avec une app Next.js). Il a été
  nettoyé par force-push. Si `fork/main` contient un commit hors-sujet, le signaler
  et demander validation avant de force-push.

### 7-2. Build des images
```bash
SANDBOXD_VERSION=0.4.1-nordineonline.<n> SANDBOXD_GIT_COMMIT=$(git rev-parse --short HEAD) docker compose build sandboxd console
docker build -t sandboxd-base:0.4.1-nordineonline.<n> -f image/Dockerfile .   # TOUJOURS si runtimed/*.go a changé
```

### 7-3. Déploiement STAGING (isolé, port 9001)
Le compose staging (`/tmp/opencode/staging/docker-compose.yml`) référence les
tags `0.4.1-nordineonline.3` (legacy). Deux options :
- soit retaguer les images : `docker tag sandboxd-X:0.4.1-nordineonline.<n> sandboxd-X:0.4.1-nordineonline.3`
- soit éditer les tags du compose staging.
Puis :
```bash
cd /tmp/opencode/staging && docker compose -p staging up -d --force-recreate sandboxd console
```
Pour appliquer un nouveau `runtimed` aux sandbox de staging :
```bash
curl -X DELETE -H "Authorization: Bearer sk_staging_220adbd056befdd584" http://127.0.0.1:9002/v1/sandboxes/<id>
curl -X POST  -H "Authorization: Bearer sk_staging_220adbd056befdd584" -H 'content-type: application/json' -d '{}' http://127.0.0.1:9002/v1/apps/<appId>/sandbox
```
(Auth staging : token API `sk_staging_220adbd056befdd584`, password console `testpass123`.)

### 7-4. Déploiement PROD
Dossier `/home/nordine/sandboxd-prod/` (projet compose `src`, API `127.0.0.1:9090`,
console sur `console.192.168.10.2.nip.io`, token `sk_oFru5excbo…`).
1. Éditer `/home/nordine/sandboxd-prod/docker-compose.yml` : remplacer les tags
   `0.4.1-nordineonline.<ancien>` par `<nouveau>` (control-plane, console, `SANDBOXD_IMAGE`).
2. `cd /home/nordine/sandboxd-prod && docker compose -p src up -d` (recrée les conteneurs).
3. Recréer les conteneurs sandbox existants (même logique que staging) pour que
   `runtimed` soit à jour, sinon le routage providers ne bouge pas.
4. Vérifier : `curl http://127.0.0.1:9090/healthz` → `ok` ;
   `GET /v1/settings` affiche la bonne version.

---

## 8. Environnement de build Go (recettes utiles)

Go tourne dans un conteneur (pas de Go local garanti sur la machine hôte) :
```bash
cd /tmp/opencode/sandboxd && docker run --rm \
  -v /tmp/opencode/sandboxd:/repo \
  -w /repo/control-plane \
  -v /home/nordine/.gocache/mod:/go/pkg/mod \
  -v /home/nordine/.gocache/build:/root/.cache/go-build \
  golang:1.22-bookworm sh -c "go build -buildvcs=false ./... && go vet -buildvcs=false ./... && go test -buildvcs=false ./..."
```

---

## 9. Points à retenir pour ne pas se perdre

- **Le moteur = OpenCode.** On l'embarque, on ne le réécrit pas. Le chat headless
  et les providers sont des couches autour, construites par sandboxd.
- **`runtimed` vit dans l'image sandbox.** Si une modif backend n'a d'effet nulle
  part, vérifier que `sandboxd-base` a été reconstruit et que le conteneur sandbox
  a été recréé (pas juste stop/start).
- **La sécurité des clés** : les clés API ne sont jamais montées dans le sandbox
  pour les tâches ; le proxy (`authproxy`) les injecte à la volée. Ne pas régresser
  sur ce point.
- **Les docs de référence** : `README.md` (section "Where the fork differs"),
  `CHANGELOG.md`, `docs/agent-auth.md` (détail provider par provider),
  `ARCHITECTURE.md`, `docs/openapi.yaml`.