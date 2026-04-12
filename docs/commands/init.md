# pilot init

Initialise pilot dans un projet nouveau ou existant.

```
pilot init [project-name] [flags]
```

---

## Description

Lance un wizard TUI pour décrire ton infrastructure et génère `pilot.yaml` : la source
de vérité de tous les environnements. Fonctionne aussi bien sur un dossier vide que
sur un projet existant (pilot détecte automatiquement le stack).

**Ne génère pas de Dockerfiles.** La génération des fichiers d'infra est faite par
ton agent IA après l'init (via `pilot_generate_dockerfile` / `pilot_generate_compose`).

---

## Flags

| Flag | Alias | Description |
|------|-------|-------------|
| `--stack` | `-s` | Override du stack (`go`, `node`, `python`, `rust`, `java`) |
| `--registry` | `-r` | Provider de registry (`ghcr`, `dockerhub`, `custom`) |
| `--yes` | `-y` | Non-interactif : accepte les défauts (pour CI / agents) |

---

## Exemples

```bash
# Nouveau projet : wizard interactif
mkdir my-app && cd my-app
pilot init my-app

# Projet existant : pilot détecte le stack
cd mon-projet-go
pilot init

# Non-interactif (CI ou agent)
pilot init my-app --stack go --registry ghcr --yes
```

---

## Ce que le wizard crée

1. **Nom du projet** (pré-rempli depuis le dossier)
2. **Services** : catalogue complet : `app`, `postgres`, `redis`, `rabbitmq`, `kafka`, `mongodb`, `storage`, `nginx`, `traefik`…
3. **Services managés** : pour chaque service eligible : `container`, `managed` (avec fournisseur), `local-only`. Pré-rempli depuis les fichiers `.env*` existants.
4. **Environnements** : `dev` + `staging`, `prod`, `test`
5. **Cible de déploiement** : `none`, `vps`, `k8s`, `aws`, `gcp`
6. **Registry** : `ghcr`, `dockerhub`, `custom`, `none`
7. **Credentials du registry** : demandés si non trouvés dans l'environnement, écrits dans `.env.local`
8. **Confirmation** : résumé avant écriture

### Fichiers générés

| Fichier | Description |
|---------|-------------|
| `pilot.yaml` | Source de vérité du projet |
| `.mcp.json` | Configuration du serveur MCP pour l'agent IA |
| `.env.example` | Variables requises par service (à copier en `.env.dev`) |
| `.env.local` | Credentials de registry (mode 600, gitignored) |
| `.gitignore` | Mis à jour avec `.pilot/`, `.env.local`, `.env.*.local` |

---

## Projet existant

Sur un projet avec des fichiers de code, pilot :
- Détecte le stack depuis `go.mod`, `package.json`, `Cargo.toml`, `pyproject.toml`…
- Analyse les fichiers `.env*` pour détecter les services managés (ex: `DATABASE_URL=...neon.tech...` → postgres/neon à confidence 1.0)
- Ne modifie aucun fichier existant sauf `pilot.yaml`

---

## Après l'init

```bash
# Copier et remplir les variables d'env
cp .env.example .env.dev

# Demander à ton agent de générer les fichiers d'infra
# Dans Claude Code : "Génère les fichiers d'infrastructure manquants"

# Démarrer
pilot up
```

---

## Voir aussi

- [Guide : Démarrer avec pilot](../guide/getting-started.md) : walkthrough complet
- [Référence pilot.yaml](../reference/pilot-yaml.md) : tous les champs
- [`pilot add`](add.md) : ajouter un service après l'init
