# pilot : Modèle de résilience

> **Document de référence — conception uniquement, pas encore implémenté**

---

## Contexte : qu'est-ce que pilot ?

**pilot** est un CLI terminal-first, opinionated et IA-natif qui accompagne le développeur
de l'initialisation d'un projet jusqu'au déploiement en production.

Sa promesse centrale : **ce qui tourne en local tourne en production, sans modification.**

pilot cible en priorité les *vibe coders* — des développeurs qui codent avec ou sans agent IA,
qui ne maîtrisent pas nécessairement les rouages du DevOps, et qui ont besoin qu'un outil
pense à leur place pour tout ce qui concerne l'infrastructure, les environnements et le déploiement.
Il s'adresse aussi aux développeurs confirmés qui veulent aller vite sans sacrifier la robustesse.

En pratique, pilot orchestre :
- la **génération** du projet (Dockerfile, compose, pilot.yaml) via un agent IA ou manuellement
- la **gestion des environnements** (dev, staging, prod) et des secrets associés
- le **déploiement** sur VPS ou cloud, avec registry d'images, SSH, et healthchecks
- le **serveur MCP** qui permet à un agent IA (Claude, Cursor…) d'utiliser pilot comme outil natif

---

## Pourquoi un modèle de résilience ?

Dans les conditions normales, lancer une app avec pilot doit être trivial.
Mais le monde réel est chaotique : ports occupés, variables manquantes, migrations qui échouent,
clés SSH mal configurées, images cassées, containers qui crashent au démarrage.

Le problème actuel : quand pilot échoue, il sort une erreur. L'utilisateur ne sait pas toujours
quoi faire. L'agent IA, lui, commence à improviser des commandes dans tous les sens —
ce qui empire souvent la situation.

**Ce qu'on cherche à construire** : un pilot qui ne laisse jamais l'utilisateur ni l'agent
dans un état ambigu. Chaque échec est diagnostiqué, classé, et résolu — automatiquement
si possible, avec des instructions exactes sinon. pilot doit être le seul interlocuteur
fiable entre l'humain ou l'agent et le système, quelle que soit la complexité du projet
ou le chaos de l'environnement.

Ce document décrit le modèle qui rend ça possible : comment les opérations sont planifiées,
exécutées, réparées et compensées — sans que l'utilisateur ni l'agent n'aient à comprendre
la mécanique sous-jacente.

---

## Principe directeur

> **La complexité technique d'un projet est le problème de pilot, pas celui de l'utilisateur.**

Un vibe coder qui lance `pilot deploy` sur un projet Go avec Postgres, migrations Prisma,
nginx et deux services interdépendants doit vivre la même expérience qu'un dev
déployant une app Node.js basique. La différence de complexité est absorbée
entièrement par pilot.

Ce principe gouverne trois dimensions :

| Dimension | Ce que voit l'utilisateur | Ce que fait pilot en coulisse |
|---|---|---|
| **Opérations** | Une commande simple | Un graphe d'étapes avec dépendances |
| **Erreurs** | Un message clair + des étapes exactes | Une taxonomie structurée à 4 niveaux |
| **pilot.yaml** | Seulement ce qui est vraiment nécessaire | Inférence automatique du reste |

---

## 1. Le modèle d'exécution hybride

### 1.1 Pourquoi hybride

Deux extrêmes existent :

- **Graphe statique pur** : chaque commande a une séquence fixe câblée dans le code.
  Prévisible mais rigide : un projet avec migrations ressemble au même code qu'un projet sans.

- **Graphe dynamique pur** : pilot calcule un graphe arbitraire depuis `pilot.yaml`.
  Puissant mais imprévisible : l'agent ne sait jamais ce qui va se passer avant de demander.

Le modèle hybride prend le meilleur des deux :

> **La forme du graphe est fixe et connue à l'avance.**
> **Le contenu de chaque nœud est calculé dynamiquement depuis le projet.**

L'agent et l'utilisateur bénéficient de la prévisibilité du statique
et de la précision du dynamique. Ils ne voient que le résultat.

---

### 1.2 Le squelette d'exécution universel

Toute opération complexe de pilot suit ce squelette, dans cet ordre invariable :

```
┌─────────────────────────────────────────────────────────────────┐
│                     SQUELETTE D'EXÉCUTION                       │
│                                                                 │
│  [1] PREFLIGHT          vérification exhaustive avant tout      │
│       │                                                         │
│  [2] PRE-HOOKS          actions configurées à exécuter avant    │  (optionnel)
│       │                                                         │
│  [3] MIGRATIONS         évolution du schéma de données          │  (optionnel)
│       │                                                         │
│  [4] BUILD / PUSH       construction et publication de l'image  │  (si applicable)
│       │                                                         │
│  [5] DEPLOY             mise en place du nouvel état cible      │
│       │                                                         │
│  [6] POST-HOOKS         actions configurées à exécuter après    │  (optionnel)
│       │                                                         │
│  [7] HEALTHCHECK        validation que l'état cible est sain    │
│                                                                 │
│  Si échec en [3..7] → PLAN DE COMPENSATION (ordre inverse)      │
└─────────────────────────────────────────────────────────────────┘
```

Les nœuds entre crochets `[ ]` sont **activés ou désactivés** selon ce que pilot
détecte dans le projet et dans `pilot.yaml`. Un nœud désactivé est skip silencieusement.

Exemples selon les projets :

```
App Node.js simple, pas de DB :
  [1] preflight → [5] deploy → [7] healthcheck

App Python + Postgres + migrations Alembic :
  [1] preflight → [3] migrations(alembic) → [4] push → [5] deploy → [7] healthcheck

App avec nginx et reload nécessaire :
  [1] preflight → [4] push → [5] deploy → [6] nginx-reload → [7] healthcheck

Monorepo, 2 services interdépendants :
  [1] preflight → [3] migrations(api) →
    service-api  : [4] push → [5] deploy → [7] health ─┐
    service-worker (attend api.healthy) : ──────────────▶ [4] push → [5] deploy → [7] health
```

La forme est toujours reconnaissable. Ce qui change c'est ce qui s'active.

---

### 1.3 Ce que pilot infère automatiquement

pilot analyse le projet et active les nœuds sans intervention manuelle.
Mais l'inférence n'est pas magique — elle est **graduée selon le niveau de confiance** :

- **Confiance haute** (signal non-ambigu) → pilot agit et **annonce** ce qu'il fait (TYPE B).
  L'utilisateur voit ce qui se passe, peut interrompre si besoin.
- **Confiance basse** (signal ambigu ou multiple) → pilot **liste et demande** (TYPE C).
  Il ne choisit jamais arbitrairement.

| Signal détecté | Confiance | Nœud activé | Ce que pilot fait |
|---|---|---|---|
| `prisma/schema.prisma` seul dans le projet | haute | [3] migrations | annonce + `npx prisma migrate deploy` |
| `alembic.ini` seul dans le projet | haute | [3] migrations | annonce + `alembic upgrade head` |
| `go-migrate` dans les dépendances | haute | [3] migrations | annonce + `migrate up` |
| `flyway.conf` dans le projet | haute | [3] migrations | annonce + `flyway migrate` |
| Plusieurs outils de migration détectés | basse | [3] migrations | demande lequel utiliser |
| Service nginx avec bind-mount `.conf` | haute | [6] post-hook | annonce + `nginx -s reload` |
| Services avec `depends_on` | haute | [5] deploy | ordre topologique respecté |
| `healthcheck:` dans compose | haute | [7] healthcheck | attend le statut `healthy` |

**Ce que l'utilisateur voit dans tous les cas :**
```
  → Migration détectée : prisma (prisma/schema.prisma)
    Commande : npx prisma migrate deploy
    Continuez avec Ctrl+C pour annuler...
```

L'inférence est transparente. Elle ne se cache pas. Si pilot se trompe,
l'utilisateur le voit avant que quoi que ce soit soit exécuté.

---

### 1.4 Multi-services et dépendances

Quand un projet a plusieurs services, pilot résout l'ordre de déploiement
depuis les déclarations `depends_on` du fichier compose. L'utilisateur
n'a pas à gérer ça dans `pilot.yaml` : il le déclare déjà dans compose.

```
# docker-compose.prod.yml  ← pilot lit ça
services:
  api:
    depends_on: [db]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 5s
      timeout: 3s
      retries: 10
  worker:
    depends_on: [api, redis]
  db: ...
  redis: ...

# pilot en déduit l'ordre de lancement :
# db → redis → api → worker
```

**Important : `depends_on` ≠ readiness.**

Docker lui-même le reconnaît : `depends_on` garantit l'ordre de démarrage,
pas que le service précédent est prêt à recevoir des requêtes.

pilot traite les deux séparément :

| Ce que pilot utilise | Pour quoi |
|---|---|
| `depends_on` | déterminer l'ordre de lancement |
| `healthcheck` dans compose | signal de readiness réel avant de passer au service suivant |

pilot attend le statut `healthy` du healthcheck avant de déployer
le service suivant dans la chaîne. Si aucun `healthcheck` n'est défini
sur un service dont d'autres dépendent → warning au preflight, car pilot
ne pourra pas garantir la readiness de ce service.

En cas de cycle détecté dans `depends_on` → erreur PILOT-CONFIG-001 avec le cycle identifié précisément.

---

## 2. Le plan de compensation

Avant d'exécuter le nœud [3] migrations (le premier nœud potentiellement irréversible),
pilot **calcule son plan de compensation complet**. Il ne commence que s'il est en mesure
de répondre à la question : *si ça échoue plus tard, que peut-on défaire ?*

### 2.1 Périmètre de la compensation

> **pilot restaure un état opérationnel cohérent pour les composants qu'il contrôle.**
> Il ne promet pas de rétablir l'état exact du monde — certains effets externes
> sont irréversibles par nature.

**Ce que pilot peut compenser :**

| Nœud | Compensation | Condition |
|---|---|---|
| [3] migrations | rollback via `rollback_command` | si `reversible: true` (défaut: true) |
| [4] push | aucune (image publiée, inoffensif) | — |
| [5] deploy | restaurer l'image précédente | image précédente connue dans state.json |
| [6] post-hooks | dépend du hook | déclaré dans pilot.yaml si besoin |
| [7] healthcheck | déclenche la compensation | c'est le déclencheur, pas une étape |

**Ce que pilot ne peut pas compenser (toujours signalé explicitement) :**

- Emails, SMS ou notifications envoyés pendant la fenêtre de déploiement
- Jobs déjà consommés depuis une queue (Kafka, RabbitMQ, SQS…)
- Webhooks déjà déclenchés vers des systèmes externes
- Invalidations de cache ou CDN déjà propagées
- Migrations destructives marquées `reversible: false`

Ces effets sont listés dans le rapport final pour que l'utilisateur ou
l'agent sache exactement ce qui ne peut pas être automatiquement défait.

### 2.2 La règle des étapes irréversibles

> **pilot ne franchit jamais une étape irréversible sans avoir établi que les étapes
> suivantes ont une forte probabilité de succès.**

Si une migration est marquée `reversible: false` et que le preflight
détecte un risque sur les étapes suivantes (image non testée, healthcheck
absent, env vars manquantes) → pilot **bloque et demande confirmation** avant
d'aller plus loin. C'est un choix TYPE C (voir section 3).

### 2.3 Exécution de la compensation

En cas d'échec :

```
Échec détecté au nœud [5] deploy
  │
  ├─ compensation [5] : restaurer image précédente    ← automatique
  ├─ compensation [3] : alembic downgrade -1          ← si reversible: true
  │
  └─ rapport final :
       composants pilot restaurés :
         ✓ image : v1.3 → v1.2 (restaurée)
         ✓ schéma : migration_008 → migration_007 (revert OK)

       effets externes non compensables :
         ✗ 3 emails de confirmation envoyés pendant la fenêtre
         ✗ webhook order.created déclenché vers Stripe

       cause de l'échec : container OOM (limit 512MB insuffisante)
       prochaine étape  : augmenter memory_limit dans docker-compose.prod.yml
```

Si la compensation elle-même échoue → pilot le signale explicitement avec
l'état exact de chaque composant, sans tenter d'improviser davantage.

---

## 3. Taxonomie des erreurs et résolution

Toute situation anormale est classée dans l'un des 4 types suivants.
Le type détermine qui agit et comment.

### Type A : Déterministe, sans risque
pilot corrige seul, log ce qu'il a fait, continue.

```
Exemples :
  → permissions SSH key incorrectes (0644) → chmod 600 automatique
  → .env absent mais .env.example présent → copie et avertit
  → image locale obsolète → docker pull automatique avant up
```

### Type B : Déterministe, impactant
pilot corrige seul mais annonce clairement ce qu'il fait.
Supporte `--dry-run` pour voir sans exécuter.

```
Exemples :
  → Docker non démarré → lance Docker, attend qu'il soit prêt
  → nginx à recharger après sync → nginx -s reload annoncé
  → réseau Docker absent → docker network create annoncé
```

### Type C : Choix requis, options connues
pilot liste les options et **attend** la réponse.
Il ne choisit jamais arbitrairement.

```
Exemples :
  → conflit de port → liste les ports libres, attend le choix
  → user SSH non configuré → demande quel user utiliser
  → migration irréversible avant deploy risqué → demande confirmation explicite
  → plusieurs targets disponibles → lequel déployer ?
```

**En terminal (humain) :**
```
  ✗  Port 8080 est occupé (nginx, pid 1234)

     Ports libres disponibles pour le service api :
       [1] 8081   ← recommandé (prochain libre)
       [2] 8082
       [3] 3001

     Votre choix [8081] :
```

**En JSON (agent IA) :**
```json
{
  "status": "awaiting_choice",
  "code": "PILOT-NET-001",
  "message": "Port 8080 is in use by nginx (pid 1234)",
  "options": [8081, 8082, 3001],
  "recommended": 8081,
  "applies_to": "environments.dev.ports.api",
  "resume_with": "pilot resume --answer 8081"
}
```

### Type D : Choix requis, options non énumérables
pilot s'arrête avec des instructions exactes. Il n'improvise jamais.

```
Exemples :
  → valeur d'une variable d'env manquante → explique quelle var, où la mettre
  → mot de passe SSH → explique comment le configurer
  → token registry expiré → explique comment le renouveler
```

---

## 4. Mécanisme de suspension

Quand pilot attend une réponse (TYPE C), l'opération est **suspendue**, pas annulée.

### Pour l'humain en terminal
pilot attend directement sur stdin. L'opération reprend après le choix
sans relancer la commande. L'état avant la suspension est préservé en mémoire.

```
$ pilot deploy
  ✓  preflight OK
  ✓  migrations OK
  ✗  conflit de port détecté

     [choix demandé : voir ci-dessus]

  → Choix saisi : 8081
  ✓  pilot.yaml mis à jour (environments.prod.ports.api: 8081)
  →  reprise du déploiement...
  ✓  deploy OK
  ✓  healthcheck OK
```

### Pour l'agent IA
pilot s'arrête et écrit l'état dans `.pilot/state.json` avec `pending_choice`.
L'agent lit l'état, répond via `pilot resume --answer <valeur>`, pilot reprend.

```
# Cycle de l'agent :
1. appelle pilot deploy
2. reçoit {"status": "awaiting_choice", ...}
3. lit les options, choisit selon son contexte
4. appelle pilot resume --answer 8081
5. reçoit {"status": "succeeded"} ou nouvelle étape
```

Ce mécanisme garantit que l'agent ne se retrouve jamais à deviner
ou à improviser des commandes système en dehors de pilot.

---

## 5. `pilot diagnose` : le snapshot exhaustif

Commande disponible à tout moment, avant ou après une opération.
Donne une vue complète et fiable de l'état du système.

```
$ pilot diagnose

─── Système ──────────────────────────────────────────
  Docker          : ✓ running (v26.1.3)
  Docker Compose  : ✓ v2.27.0
  Go              : ✓ 1.23.4
  Connexion réseau: ✓

─── Projet ───────────────────────────────────────────
  pilot.yaml       : ✓ valide
  .env.dev        : ✓ 12 vars, 0 vide
  .env.prod       : ✗ 2 vars vides  →  DATABASE_URL, REDIS_URL
  Dockerfile      : ✓ pattern ARG-only (pas d'ENV qui bake les valeurs)
  compose.dev.yml : ✓ env_file présent sur tous les services

─── Migrations ───────────────────────────────────────
  Outil détecté   : prisma
  État            : 2 migrations appliquées, 1 en attente
  Réversible      : ✓ (rollback_command disponible)

─── Ports locaux (env: dev) ──────────────────────────
  8080 (api)      : ✓ libre
  5432 (db)       : ✗ OCCUPÉ : postgres externe (pid 5678)
  6379 (redis)    : ✓ libre

─── Registry ─────────────────────────────────────────
  ghcr.io         : ✓ authentifié (mouhamedsylla)
  joignabilité    : ✓ (<50ms)

─── SSH / VPS (target: vps-prod) ─────────────────────
  host            : 1.2.3.4   user: deploy
  clé SSH         : ~/.ssh/id_pilot  ✓ (permissions 600)
  connectivité    : ✓ joignable (120ms)
  Docker (VPS)    : ✓ running
  ports VPS       : 80→nginx  443→nginx  5432→postgres
                    8080 → LIBRE

─── Build args ───────────────────────────────────────
  VITE_API_URL    : ✓ dans build_args
  VITE_APP_ENV    : ✗ absent de build_args → sera vide dans l'image

─── Git ──────────────────────────────────────────────
  branche         : main
  état            : dirty (3 fichiers modifiés)
  dernier commit  : abc1234  "feat: add auth"

─── État pilot ────────────────────────────────────────
  dernière opération : pilot deploy  (succès, il y a 2h)
  opération en cours : aucune
  choix en attente   : aucun
```

Chaque ligne est `✓` ou `✗` avec la raison précise.
Utilisable par l'agent pour établir un point de départ fiable
avant de lancer n'importe quelle opération.

---

## 6. `.pilot/state.json` : mémoire persistante

pilot maintient un fichier d'état pour ne pas redécouvrir à chaque exécution
ce qui a déjà été fait ou ce qui est en attente.

```json
{
  "schema_version": 1,
  "active_env": "dev",

  "last_operation": {
    "command": "pilot deploy",
    "env": "prod",
    "status": "succeeded",
    "started_at": "2026-03-31T16:00:00Z",
    "completed_at": "2026-03-31T16:03:12Z"
  },

  "last_success_per_command": {
    "deploy": "2026-03-31T16:03:12Z",
    "push":   "2026-03-31T15:58:00Z",
    "up":     "2026-03-31T10:22:00Z"
  },

  "pending_choice": null,

  "deployed": {
    "prod": {
      "image": "ghcr.io/mouhamedsylla/mon-projet:abc1234",
      "deployed_at": "2026-03-31T16:03:12Z",
      "previous_image": "ghcr.io/mouhamedsylla/mon-projet:abc0000"
    }
  },

  "known_containers": {
    "dev": ["api", "db", "redis"]
  }
}
```

**`pending_choice` non-null** indique à l'agent qu'il doit répondre
avant de lancer toute autre opération. Exemple :

```json
"pending_choice": {
  "code": "PILOT-NET-001",
  "prompt": "Port 8080 in use. Choose available port for api.",
  "options": [8081, 8082, 3001],
  "recommended": 8081,
  "applies_to": "environments.dev.ports.api",
  "operation_suspended": "pilot deploy",
  "suspended_at": "2026-03-31T16:05:00Z"
}
```

---

## 7. `pilot.yaml` : principe de minimalité

> **pilot.yaml exprime l'intention. pilot infère l'exécution.**

La règle : un champ n'entre dans `pilot.yaml` que s'il remplit l'une des deux conditions :
1. pilot ne peut pas le déduire automatiquement du projet
2. L'utilisateur veut explicitement déroger au comportement par défaut

### Ce que pilot.yaml doit contenir (non-déductible)

```yaml
apiVersion: pilot/v1

project:
  name: mon-projet
  stack: go              # pilot peut le détecter mais c'est une intention explicite

registry:
  provider: ghcr
  image: ghcr.io/mouhamedsylla/mon-projet

environments:
  dev:
    compose_file: docker-compose.dev.yml   # plusieurs fichiers possibles → choix explicite
    env_file: .env.dev

  prod:
    target: vps-prod
    compose_file: docker-compose.prod.yml
    env_file: .env.prod

targets:
  vps-prod:
    host: 1.2.3.4
    user: deploy          # pas toujours "deploy" → explicite
    key: ~/.ssh/id_pilot   # ou: password: true  (demandera à l'exécution)
    port: 22
```

C'est tout ce qui est **nécessaire**. Le reste est inféré.

### Ce que pilot infère sans configuration

| Élément | Comment pilot le déduit |
|---|---|
| Outil de migrations | présence de `prisma/`, `alembic.ini`, `go-migrate`, etc. |
| Ordre de déploiement | `depends_on` dans le fichier compose |
| Ports à vérifier | `ports:` dans le fichier compose |
| nginx à recharger | services image `nginx` avec bind-mount `*.conf` |
| Vars compile-time | préfixes `VITE_`, `NEXT_PUBLIC_`, `REACT_APP_`, etc. |
| Healthcheck | `healthcheck:` dans le fichier compose |
| Stack language | fichiers présents (`go.mod`, `package.json`, `requirements.txt`, etc.) |

### Ce qu'on ajoute uniquement pour déroger au défaut

```yaml
# Seulement si on veut changer le comportement par défaut :

environments:
  prod:
    migrations:
      reversible: false     # défaut: true : à déclarer si la migration est destructive
      timeout: 300          # défaut: 120s

registry:
  build_args:               # seulement si pilot n'auto-détecte pas correctement
    - VITE_API_URL
```

### Ce qui n'a pas sa place dans pilot.yaml

- La liste des services (c'est dans compose)
- Les dépendances entre services (c'est dans compose)
- La commande de migration exacte (pilot la détecte)
- Les variables d'env et leurs valeurs (c'est dans `.env.*`)
- La configuration de l'application (c'est dans le projet)

---

## 8. Ce que voit l'utilisateur

L'ensemble du modèle ci-dessus est invisible. Ce que vit le dev :

### Scénario : premier déploiement, port occupé sur le VPS

```
$ pilot deploy

  → Analyse du projet...
  → Plan : migrations (prisma) → push → deploy → healthcheck

  ✓  preflight OK
  ✓  migration appliquée  (20260331_add_users_table)

  ✗  Port 8080 est occupé sur vps-prod (nginx, pid 1234)

     Ports disponibles sur vps-prod :
       [1] 8081  ← recommandé
       [2] 3001

     Votre choix [8081] : 1

  ✓  pilot.yaml mis à jour  (prod.ports.api: 8081)
  ✓  image construite et pushée  (ghcr.io/.../mon-projet:abc1234)
  ✓  container déployé
  ✓  healthcheck OK  (api répond en 230ms)

  Déployé sur vps-prod  →  http://1.2.3.4:8081
```

### Scénario : échec healthcheck avec compensation

```
$ pilot deploy

  ✓  preflight OK
  ✓  migration appliquée

  ✓  image pushée
  ✓  container déployé
  ✗  healthcheck échoué  (api ne répond pas après 60s)

  → Compensation en cours...
  ✓  image précédente restaurée  (abc0000)
  ✓  migration revertée

  État du système : identique à avant le déploiement.

  Cause probable : container redémarre en boucle (OOM)
  Diagnostic     : pilot logs api --env prod
  Fix suggéré    : augmenter la limite mémoire dans docker-compose.prod.yml
```

L'utilisateur n'a pas eu à comprendre ce qu'est un plan de compensation,
une transaction de déploiement ou une taxonomie d'erreurs.
Il a vu ce qui s'est passé et sait exactement quoi faire ensuite.

---

## 9. Ce que voit l'agent IA

L'agent reçoit des structures JSON à chaque étape, sans ambiguité :

### Avant l'exécution : le plan
```json
{
  "operation": "deploy",
  "env": "prod",
  "plan": [
    { "step": 1, "name": "preflight" },
    { "step": 2, "name": "migrations", "tool": "prisma", "reversible": true },
    { "step": 3, "name": "push" },
    { "step": 4, "name": "deploy" },
    { "step": 5, "name": "healthcheck" }
  ],
  "compensation_plan": {
    "step_4": "restore previous image",
    "step_2": "prisma migrate rollback"
  }
}
```

### En cas de choix requis
```json
{
  "status": "awaiting_choice",
  "code": "PILOT-NET-001",
  "severity": "BLOCKING",
  "message": "Port 8080 in use on vps-prod (nginx, pid 1234)",
  "options": [8081, 3001],
  "recommended": 8081,
  "applies_to": "environments.prod.ports.api",
  "resume_with": "pilot resume --answer 8081"
}
```

### En cas d'échec guidé
```json
{
  "status": "guided_failure",
  "code": "PILOT-RUNTIME-003",
  "message": "Healthcheck failed after 60s. Container restarting (OOM).",
  "compensation": "completed",
  "system_state": "restored to pre-deploy",
  "remediation": {
    "steps": [
      "pilot logs api --env prod",
      "Increase memory limit in docker-compose.prod.yml",
      "pilot deploy"
    ]
  }
}
```

L'agent ne reçoit jamais une erreur vague. Il sait toujours :
l'état actuel du système, pourquoi ça a échoué, quoi faire ensuite.

---

## 10. Machine à états de pilot

```
IDLE
  │
  ├─▶ PREFLIGHTING
  │       │
  │       ├─ TYPE A/B détecté → RECOVERING → retour PREFLIGHTING
  │       ├─ TYPE C détecté   → AWAITING_CHOICE
  │       │       │
  │       │       └─ réponse reçue → retour PREFLIGHTING
  │       └─ OK
  │           │
  ├─▶ EXECUTING
  │       │
  │       ├─ TYPE A/B détecté → RECOVERING → retour EXECUTING
  │       ├─ TYPE C détecté   → AWAITING_CHOICE → réponse → retour EXECUTING
  │       ├─ TYPE D détecté   → GUIDED_FAILURE
  │       └─ OK
  │           │
  ├─▶ SUCCEEDED
  │
  └─▶ GUIDED_FAILURE
```

**Règle absolue** : pilot ne se termine que dans l'état `SUCCEEDED`
ou dans l'état `GUIDED_FAILURE`. Jamais dans un état indéterminé.

---

## Récapitulatif des commandes liées à ce modèle

| Commande | Rôle |
|---|---|
| `pilot preflight` | Exécute uniquement la phase [1] et sort le résultat |
| `pilot diagnose` | Snapshot exhaustif de l'état du système |
| `pilot resume` | Reprend une opération suspendue (après un choix TYPE C) |
| `pilot resume --answer <val>` | Répond à un pending_choice et reprend (pour l'agent) |
| `pilot rollback` | Exécute manuellement le plan de compensation de la dernière opération |
| `pilot status` | État des containers + dernière opération connue |

---

## 11. Évolutions planifiées de la surface de commandes

Ce modèle de résilience implique des ajouts et des retraits par rapport à l'état actuel de pilot.
Rien n'est implémenté ici — c'est le plan de référence.

### À ajouter

#### `pilot validate` — analyse statique offline
Vérifie la cohérence du projet sans aucune dépendance runtime (pas de Docker, pas de réseau).
Peut tourner sur une machine fraîche, en CI, avant toute installation.
Cible : Dockerfile antipatterns, `pilot.yaml` valide, `.env.*` cohérents, `.gitignore` correct.
Différent de `pilot preflight` qui nécessite Docker et le réseau.

#### `pilot env diff <env1> <env2>` — parité entre environnements
Affiche les variables présentes dans un environnement et absentes de l'autre,
les ports qui diffèrent, les services qui ne sont pas dans les deux compose files.
Colmate le trou classique "ça marche en dev mais plante en prod pour une variable manquante".

#### `pilot secrets check [--env <env>]` — valider sans exposer
Vérifie que chaque secret référencé dans `pilot.yaml` existe et est non-vide
dans le provider configuré. Aucune valeur affichée — juste existence + non-vide.
S'intègre au preflight avant chaque deploy.

#### `--dry-run` sur `pilot deploy` et `pilot push`
Affiche le plan complet (étapes, compensation prévue, ce qui va changer)
sans rien exécuter. Utile pour l'agent avant d'agir, et pour l'humain prudent avant prod.

#### `pilot clean` — nettoyage ciblé du projet
Supprime les images locales obsolètes, containers stopped, volumes orphelins
appartenant à ce projet spécifiquement. Plus chirurgical que `docker system prune`
qui touche tous les projets de la machine.

#### `pilot env push [--env <env>]` — mettre à jour les vars sans redéployer
Synchronise le fichier `.env.*` vers le VPS et redémarre les services affectés,
sans rebuilder l'image. Comble le vide entre `pilot sync` (fichiers config)
et `pilot deploy` (image complète) pour le cas "je veux juste changer LOG_LEVEL".

```
pilot sync      → fichiers statiques (nginx.conf, certs…)
pilot env push  → variables d'environnement + restart des services
pilot deploy    → nouvelle image + tout le reste
```

#### `pilot init --adopt` — adopter un projet existant
Génère `pilot.yaml` depuis un projet qui a déjà un Dockerfile et un docker-compose,
au lieu de créer de zéro. Réduit la friction pour les projets qui veulent rejoindre
pilot sans tout refaire.

---

### À retirer

#### `pilot_config_get` / `pilot_config_set` — outils MCP
Permettre à un agent IA de lire et écrire `pilot.yaml` directement est trop risqué.
Une correction silencieuse d'un champ peut en casser un autre.
`pilot.yaml` reste sous contrôle humain — l'agent peut suggérer, l'humain applique.

#### `pilot_generate_k8s` — outil MCP
Le provider Kubernetes n'est pas encore implémenté. Exposer un outil MCP
pour quelque chose qui ne fonctionne pas crée de fausses attentes.
À réintroduire quand k8s sera réellement opérationnel.

---

### À fusionner / absorber

#### `pilot setup` → dans `pilot preflight --fix`
`pilot setup` n'ajoute l'utilisateur VPS au groupe docker. C'est un TYPE B :
pilot peut le détecter lors du preflight et le corriger automatiquement.
Pas besoin d'une commande de premier niveau pour ça.

#### `pilot history` → dans `pilot status --history`
`pilot history` est une vue temporelle de `state.json`. Ce n'est pas un concept
distinct de `pilot status` — c'est la même chose avec un filtre temporel.
`pilot status` pour l'état courant, `pilot status --history` pour le passé.

#### `pilot context` → sous `pilot mcp context`
`pilot context` n'a de sens que dans un contexte MCP. Exposée au même niveau
que `pilot deploy` dans `--help`, elle crée de la confusion pour l'utilisateur humain.
À ranger sous le namespace `pilot mcp` pour clarifier que c'est une commande agent.

---

*Ce document est la référence de conception pour l'implémentation du modèle de résilience de pilot.*
*Aucun code ne doit être écrit avant validation de ce document.*
