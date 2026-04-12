# pilot update

Met à jour pilot vers la dernière version.

```
pilot update [flags]
```

---

## Description

Vérifie s'il existe une nouvelle version sur GitHub et installe la mise à jour.

Stratégie (dans l'ordre) :
1. Si `go` est disponible → `go install github.com/mouhamedsylla/pilot@latest`
2. Sinon → ré-exécute le script d'installation officiel via `curl | sh`

---

## Flags

| Flag | Description |
|------|-------------|
| `--check` | Vérifie seulement la version disponible, sans mettre à jour |
| `--force` | Met à jour même si la version actuelle est déjà la dernière |

---

## Exemples

```bash
# Vérifier sans mettre à jour
pilot update --check

# Mettre à jour
pilot update

# Forcer la réinstallation
pilot update --force
```

---

## Comportement

```bash
pilot update

Current version : v0.4.1
Latest version  : v0.5.0
Installing via go: github.com/mouhamedsylla/pilot@v0.5.0
✓ pilot updated to v0.5.0
  Restart your shell or run: hash -r
```

Si déjà à jour :

```bash
pilot update

Current version : v0.5.0
Latest version  : v0.5.0
✓ Already on the latest version.
```

---

## Variables d'environnement

`GITHUB_TOKEN` est utilisé si disponible pour éviter le rate limiting de l'API GitHub
lors de la vérification de version. Ce n'est pas requis mais recommandé en CI.

---

## Voir aussi

- [GitHub Releases](https://github.com/mouhamedsylla/pilot/releases) : notes de version
