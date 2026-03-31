

| pilot *Dev Environment as Code* Du projet local au cloud, en une commande. |
| :---: |

| Auteur | Mouhamed SYLLA |
| :---: | :---: |
| **Date** | Mars 2026 |
| **Version** | 0.1 — Exploration / Conception |

*Document de conception interne. Sujet à évolution.*

# **1\. Vision du projet**

## **1.1 Le problème**

L'ère du vibe coding a radicalement changé la façon dont le code est produit. Un développeur solo ou une petite équipe peut aujourd'hui générer une application complète en quelques heures grâce aux agents IA. Le vrai goulot d'étranglement n'est plus l'écriture du code : c'est le fossé entre le moment où ce code existe et le moment où il tourne correctement en production.

Ce fossé se manifeste de façon répétitive à travers les mêmes problèmes :

* **Aucune structure de projet standard.** Chaque développeur réinvente son organisation de fichiers, ses Dockerfiles et ses scripts de déploiement à chaque nouveau projet.

* **Gestion des environnements artisanale.** Les fichiers .env se multiplient, se perdent et ne sont jamais vraiment isolés entre les environnements de développement, de staging et de production.

* **Le déploiement reste un moment de friction.** Passer du local à un VPS ou à un cloud provider nécessite des heures de configuration DevOps que la majorité des développeurs n'ont pas envie de refaire à chaque projet.

* **Les outils existants supposent une expertise préalable.** Tilt, Skaffold et DevSpace sont puissants mais supposent une maîtrise de Kubernetes et n'interviennent pas au moment de l'initialisation du projet.

* **Aucun outil n'est conçu pour les agents IA.** Aucun des outils existants n'expose d'interface MCP. Ils ne peuvent pas être pilotés nativement par Claude, Cursor ou Claude Code.

## **1.2 La proposition de valeur**

| pilot en une phrase |
| :---- |
| Un outil CLI terminal-first, opiniated et IA-natif qui accompagne le développeur de l'initialisation du projet jusqu'au déploiement en production, en rendant l'environnement local identique à l'infrastructure distante, que celle-ci soit un VPS bare-metal ou un cluster managé chez AWS, GCP, Azure ou DigitalOcean. |

Trois principes fondateurs guident toutes les décisions de conception de pilot :

1. **Local-first, cloud-portable.** Ce qui tourne en local tourne en production sans modification. La configuration est le même fichier pilot.yaml, les mêmes images Docker, les mêmes variables d'environnement structurées de la même façon.

2. **Opiniated par défaut, extensible si nécessaire.** pilot prend des décisions raisonnables à la place du développeur lors de l'initialisation, mais reste entièrement configurable pour les cas avancés qui le requièrent.

3. **IA-natif dès la conception.** pilot expose un serveur MCP complet. Claude, Cursor, Claude Code et tout agent compatible MCP peuvent initialiser, déployer, monitorer et déboguer un projet sans quitter le chat.

# **2\. Fonctionnalités principales**

## **2.1 Interface CLI**

pilot s'utilise exclusivement en ligne de commande. La surface de commandes est volontairement minimaliste pour la première version, avec six commandes principales qui couvrent l'intégralité du cycle de vie d'un projet.

| Commande | Description | Comportement clé |
| :---- | :---- | :---- |
| pilot init | Initialise un nouveau projet | Génère la structure complète, le pilot.yaml, les Dockerfiles et les configs CI après 4 questions |
| pilot env use | Switche l'environnement actif | Change les variables, les ports, les volumes et le registry cible en une commande atomique |
| pilot up | Lance l'environnement local | Docker Compose ou k3s selon la config, hot reload, logs unifiés colorés par service |
| pilot push | Build et pousse l'image | Build multi-arch si configuré, tag automatique par version, push vers le registry du provider |
| pilot deploy | Déploie sur la cible distante | SSH plus docker compose ou kubectl selon la cible, rollback intégré en cas d'échec |
| pilot sync | Synchronise la config locale vers remote | Copie pilot.yaml et les configurations vers le VPS ou le cluster, opération idempotente |
| pilot status | État complet du projet | Containers locaux, services distants, derniers déploiements, sortie JSON avec le flag \--json |
| pilot logs | Logs d'un service | Local ou distant selon l'env actif, supporte \--follow et \--since pour le suivi en temps réel |
| pilot mcp serve | Démarre le serveur MCP | Transport stdio, utilisé par les clients IA pour piloter pilot programmatiquement |

## **2.2 Le fichier pilot.yaml**

Toute la connaissance d'un projet pilot est encodée dans un seul fichier versionné à la racine du dépôt. Ce fichier est la source de vérité unique pour tous les environnements et toutes les cibles de déploiement. Il est généré automatiquement par pilot init et évolue avec le projet.

| Exemple de pilot.yaml commenté |
| :---- |
| apiVersion: pilot/v1 project: name: mon-projet stack: go language\_version: "1.23" registry: provider: ghcr image: ghcr.io/user/mon-projet environments: dev: compose\_file: docker-compose.dev.yml env\_file: .env.dev ports: { api: 8080, db: 5432 } staging: target: vps-staging compose\_file: docker-compose.staging.yml prod: target: cloud-gcp orchestrator: k8s namespace: production targets: vps-staging: type: vps host: 192.168.1.10 user: deploy key: \~/.ssh/id\_pilot cloud-gcp: type: gcp project: my-gcp-project region: europe-west1 cluster: prod-cluster orchestrator: default: compose |

## **2.3 Gestion des environnements**

pilot traite les environnements de développement, staging et production comme des citoyens de première classe. Chaque environnement est complètement isolé : variables d'environnement dans des fichiers dédiés, mappings de ports distincts pour éviter les conflits, volumes Docker namespaced par projet et par environnement, et registry cible différent selon le contexte.

L'isolation est également appliquée aux secrets : pilot s'intègre nativement avec les gestionnaires de secrets des cloud providers, de sorte que les credentials de production ne transitent jamais dans pilot.yaml ni dans le dépôt Git.

# **3\. Orchestration**

## **3.1 Choisir le bon niveau de complexité**

pilot ne force pas Kubernetes sur tous les projets. L'orchestrateur est sélectionné en fonction de la réalité du projet, en appliquant une règle simple : utiliser l'outil le moins complexe qui répond au besoin.

| Orchestrateur | Usage recommandé | Complexité opérationnelle |
| :---- | :---- | :---- |
| Docker Compose | Dev solo, APIs simples, monolithes, projets de moins de cinq services | Minimale. Un fichier YAML, une commande. |
| k3s (Lightweight K8s) | Petites équipes, microservices, VPS en production, besoin de scaling contrôlé | Faible. Kubernetes complet en un binaire de moins de 100 Mo. |
| Kubernetes standard | Cloud providers managés (GKE, EKS, AKS), production critique, équipes DevOps dédiées | Déléguée au cloud provider. |

## **3.2 Docker Compose, mode par défaut**

Pour la grande majorité des projets, Docker Compose est l'orchestrateur recommandé. pilot génère automatiquement trois fichiers Compose optimisés lors de l'initialisation du projet, un pour chaque environnement.

* **docker-compose.dev.yml** avec hot reload, volumes de développement montés et ports exposés pour le debug.

* **docker-compose.staging.yml** sans volumes de développement, avec healthchecks et restart policies adaptés.

* **docker-compose.prod.yml** hardened, avec images taggées précisément (jamais latest), ressources limitées et logs vers stdout uniquement.

## **3.3 k3s, orchestration légère pour la production sur VPS**

k3s est Kubernetes allégé, packagé en un seul binaire de moins de 100 Mo. C'est le choix naturel pour déployer un orchestrateur sérieux sur un VPS sans les coûts d'un cluster managé. pilot automatise son installation et sa configuration via SSH.

| Exemple de déploiement k3s sur VPS |
| :---- |
| pilot deploy staging \# pilot se connecte en SSH sur le VPS cible \# Installe k3s automatiquement si absent \# Copie les manifests Kubernetes générés \# Applique les manifests avec kubectl apply \# Vérifie que tous les pods passent en état Running \# Affiche l'URL de l'application déployée |

## **3.4 Kubernetes standard sur cloud provider**

Pour les cibles de type cloud provider, pilot délègue l'orchestration au cluster Kubernetes managé existant. Il se concentre sur le pipeline build, push et deploy, en s'authentifiant via les CLI natifs des providers ou via les credentials configurés dans pilot.yaml.

pilot supporte les stratégies de déploiement Rolling Update, Blue/Green et Canary, ainsi que les Ingress controllers populaires comme nginx-ingress, Traefik et les load balancers natifs des cloud providers.

# **4\. Support des cloud providers**

## **4.1 Architecture multi-cloud**

pilot abstrait les différences entre cloud providers derrière une interface uniforme. La commande pilot deploy fonctionne de la même façon que la cible soit un VPS bare-metal, GCP ou AWS. Seule la configuration du bloc targets dans pilot.yaml change.

Le support des providers est implémenté via un système de drivers pluggables en Go. Chaque driver implémente la même interface Provider, ce qui garantit une expérience cohérente quelle que soit la cible choisie.

## **4.2 Providers supportés en version 1**

| Provider | Services utilisés | Fonctionnalités pilot |
| :---- | :---- | :---- |
| VPS / Bare-metal | SSH avec Docker ou k3s | Deploy via SSH, installation automatique de Docker et k3s, synchronisation de config |
| AWS | ECR, ECS / EKS, Secrets Manager | Push vers ECR, deploy sur ECS Fargate ou EKS, injection de secrets via AWS SM |
| GCP | Artifact Registry, GKE, Cloud Run, Secret Manager | Push vers AR, deploy sur GKE ou Cloud Run serverless, secrets via GCP SM |
| Azure | ACR, AKS, Container Apps, Key Vault | Push vers ACR, deploy sur AKS ou Container Apps, secrets via Key Vault |
| DigitalOcean | DOCR, DOKS, App Platform | Push vers DOCR, deploy sur DOKS ou App Platform selon la config |
| Hetzner | Hetzner Cloud avec k3s | Provisioning de serveurs, installation k3s automatisée. Option économique recommandée. |

## **4.3 Registries d'images supportés**

| Registry | Configuration dans pilot.yaml |
| :---- | :---- |
| GitHub Container Registry | provider: ghcr — authentification via la variable GITHUB\_TOKEN |
| Docker Hub | provider: dockerhub — via DOCKER\_USERNAME et DOCKER\_PASSWORD |
| AWS ECR | provider: ecr — via rôle IAM ou la commande aws configure |
| GCP Artifact Registry | provider: gcr — via gcloud auth configure-docker |
| Azure Container Registry | provider: acr — via az acr login |
| DigitalOcean Registry | provider: docr — via doctl registry login |
| Registry privé custom | provider: custom avec l'URL registry.mondomaine.com |

## **4.4 Gestion des secrets par provider**

Les secrets de production ne transitent jamais en clair dans pilot.yaml. Seules des références sont stockées dans le fichier de configuration. pilot s'intègre avec les gestionnaires de secrets natifs de chaque provider via une commande dédiée.

* pilot secrets inject \--env prod résout les références aux secrets et les injecte dans l'environnement cible au moment du déploiement.

* **AWS Secrets Manager, GCP Secret Manager et Azure Key Vault** sont supportés dès la version 1\.

* Pour le développement local, les fichiers .env restent utilisables, mais ne sont jamais commités dans le dépôt Git.

# **5\. Intégration IA et serveur MCP**

## **5.1 Principe de conception**

pilot est conçu comme un outil IA-natif dès sa conception. Il implémente le protocole MCP (Model Context Protocol) d'Anthropic, ce qui permet à tout agent IA compatible de piloter pilot programmatiquement via un protocole standardisé. Le serveur MCP est intégré directement dans le binaire pilot, sans processus séparé, sans port réseau, sans infrastructure supplémentaire.

Les clients IA lancent le processus automatiquement en subprocess via le transport stdio. Il suffit d'ajouter un fichier .mcp.json à la racine du projet pour que Claude Code, Cursor ou tout autre agent le détecte et puisse l'utiliser.

| Configuration client (Claude Code, Cursor) |
| :---- |
| { "mcpServers": { "pilot": { "command": "pilot", "args": \["mcp", "serve"\], "cwd": "${workspaceFolder}" } } } |

## **5.2 Tools MCP exposés**

| Tool MCP | Description | Paramètres principaux |
| :---- | :---- | :---- |
| pilot\_init | Initialise un projet complet | stack, db, envs, registry, orchestrator |
| pilot\_env\_switch | Change l'environnement actif | env (dev, staging ou prod) |
| pilot\_up | Lance l'environnement local | env, services à démarrer |
| pilot\_status | Retourne l'état JSON complet | env optionnel pour filtrer |
| pilot\_logs | Retourne les logs d'un service | service, lines, follow, since |
| pilot\_push | Build et pousse l'image vers le registry | env, tag, registry cible |
| pilot\_deploy | Déploie sur une cible distante | env, target, strategy |
| pilot\_rollback | Revient à la version précédente | env, target, version optionnelle |
| pilot\_sync | Synchronise la config locale vers remote | target |
| pilot\_config\_get | Lit pilot.yaml et retourne du JSON | key en dot-notation |
| pilot\_config\_set | Modifie une valeur dans pilot.yaml | key et value |
| pilot\_secrets\_inject | Injecte les secrets dans l'environnement | env, provider |

## **5.3 Exemples concrets d'interactions IA vers pilot**

Les trois scénarios suivants illustrent comment un agent IA pilote pilot dans des situations réelles de développement.

| Scénario 1 : Initialisation de projet par un agent |
| :---- |
| Le développeur écrit dans Claude Code : "Crée un projet Go avec PostgreSQL, déploiements sur GCP" Claude Code appelle pilot\_init avec les paramètres suivants : stack: go, db: postgres, envs: dev / staging / prod registry: gcr, orchestrator: k8s, cloud\_target: gcp Résultat : projet complet généré en quelques secondes. pilot.yaml configuré, Dockerfiles créés, manifests Kubernetes générés, workflows GitHub Actions ajoutés. |

| Scénario 2 : Déploiement assisté par l'agent |
| :---- |
| Le développeur écrit dans Claude : "Les tests passent, déploie la v2.3 en staging" Claude appelle séquentiellement : 1\. pilot\_push avec env staging et tag v2.3 2\. pilot\_deploy avec env staging et target vps-staging 3\. pilot\_status avec env staging 4\. pilot\_logs avec service api et les 50 dernières lignes Claude lit le JSON retourné et confirme : "v2.3 est en staging. API répond en 45ms, tous les healthchecks sont verts." |

| Scénario 3 : Diagnostic et rollback automatique |
| :---- |
| Le développeur écrit dans Claude Code : "Quelque chose cloche en prod, regarde les logs" Claude appelle : 1\. pilot\_status avec env prod Résultat : 2 pods sur 3 en état crashlooping 2\. pilot\_logs avec service api et 200 lignes Résultat : OOMKilled détecté dans les logs 3\. pilot\_rollback avec env prod et target cloud-gcp Claude rapporte : "3 pods crashaient (OOMKilled). Rollback effectué vers v2.2. La prod est stable. Je recommande d'augmenter memory\_limit dans pilot.yaml avant le prochain déploiement de v2.3." |

# **6\. Architecture technique**

## **6.1 Stack technologique**

| Composant | Choix et justification |
| :---- | :---- |
| Langage principal | Go 1.23. Binaire unique sans dépendances, performance native, écosystème CLI mature. kubectl, docker CLI et gh CLI sont tous écrits en Go. |
| Framework CLI | Cobra avec Viper. Standard de l'industrie pour les CLIs Go. Gestion des sous-commandes, des flags et des fichiers de config. |
| TUI interactive | Bubbletea de Charm. Pour les prompts interactifs de pilot init et l'affichage des logs en temps réel. |
| Serveur MCP | Implémentation Go du protocole MCP. Transport stdio, sérialisation JSON-RPC 2.0. |
| Docker SDK | docker/docker (client Go officiel). Interaction directe avec le daemon Docker local. |
| SSH et Remote | golang.org/x/crypto/ssh. Connexions sécurisées vers les VPS pour pilot deploy et pilot sync. |
| Format de config | YAML avec schema versionné (apiVersion: pilot/v1). Validation à la lecture avec go-yaml v3. |
| Cloud SDKs | AWS SDK for Go v2, Google Cloud Go, Azure SDK for Go. Un driver par provider. |
| Tests | Go test standard avec testcontainers-go pour les tests d'intégration. |

## **6.2 Structure du repository**

pilot/

├── cmd/                    \# Commandes Cobra (init, env, up, push, deploy...)

├── internal/

│   ├── config/             \# Parsing et validation de pilot.yaml

│   ├── docker/             \# Wrapper du Docker SDK Go

│   ├── orchestrator/       \# Interfaces compose, k3s, k8s

│   ├── providers/          \# Drivers cloud (aws, gcp, azure, do, vps)

│   ├── registry/           \# Gestion des image registries

│   ├── secrets/            \# Abstraction des secret managers

│   └── mcp/                \# Serveur MCP et tools exposés

├── pkg/                    \# Packages exportables (SDK pilot)

├── templates/              \# Templates de scaffold

├── scripts/                \# Scripts CI et release

└── docs/                   \# Documentation

## **6.3 Distribution**

* **Binaire unique.** Releases via GitHub pour Linux (amd64 et arm64), macOS (Intel et Apple Silicon) et Windows.

* **Homebrew tap.** Installation via brew install pour les utilisateurs macOS et Linux.

* **Script d'installation.** Une commande curl pour les environnements CI et les VPS sans gestionnaire de paquets.

* **Image Docker.** ghcr.io/pilot-dev/pilot:latest pour l'usage en CI/CD sans installation locale.

# **7\. Roadmap**

## **7.1 Version 0 — MVP (1 à 2 mois)**

L'objectif de la v0 est de construire un outil que l'auteur utilise lui-même sur ses projets réels. Le critère de succès est simple : pilot remplace-t-il le setup manuel habituel de façon fiable ?

* **pilot init** avec scaffold pour Go et Node.js, Docker Compose uniquement, envs dev, staging et prod.

* **pilot up / down** comme wrapper docker compose avec logs colorés et unifiés par service.

* **pilot env use** avec switch d'environnement et isolation des variables.

* **pilot deploy** avec déploiement VPS via SSH et docker compose.

* **pilot.yaml v1** avec schema minimal validé à la lecture.

## **7.2 Version 1 — Production ready (3 à 4 mois)**

* **Serveur MCP complet** avec tous les tools décrits en section 5.2.

* **Support k3s** pour l'orchestration légère sur VPS en production.

* **Registries intégrés** GHCR, Docker Hub et ECR dès la v1.

* **Drivers GCP et AWS** premiers providers cloud supportés nativement.

* **pilot push** avec build multi-arch, tag automatique et push vers le registry configuré.

* **Gestion des secrets** via AWS Secrets Manager et GCP Secret Manager.

## **7.3 Version 2 — Écosystème (6 à 12 mois)**

* **Kubernetes standard** sur GKE, EKS et AKS avec Helm intégré.

* **Azure et DigitalOcean** drivers complets pour ces deux providers.

* **pilot rollback automatique** déclenché sur détection d'anomalie dans les logs ou les healthchecks.

* **Marketplace de templates** pour partager des configurations pilot.yaml entre projets et équipes.

* **Dashboard TUI** avec vue temps réel de tous les environnements via Bubbletea.

* **Système de plugins** pour étendre pilot vers des stacks non supportées nativement.

# **8\. Positionnement et différenciation**

## **8.1 Comparaison avec les outils existants**

| Critère | pilot | Tilt, Skaffold, DevSpace |
| :---- | :---- | :---- |
| Intervient à l'initialisation du projet | Oui, scaffold complet | Non, suppose une infra existante |
| Gestion des envs intégrée | Oui, dev, staging et prod natifs | Partielle, configs manuelles |
| Support VPS bare-metal | Oui, SSH natif | Non, orienté Kubernetes |
| Support cloud providers | Oui, AWS, GCP, Azure, DO | Via intégrations tierces |
| Serveur MCP intégré | Oui, natif dès la v1 | Non |
| Courbe d'apprentissage | Faible, 4 commandes suffisent | Élevée, Kubernetes requis |
| Binaire unique sans dépendances | Oui | Non, dépendances externes |
| Licence | MIT, open source | Apache 2.0 ou MIT selon l'outil |

## **8.2 Positionnement**

| Vibe coding ready. |
| :---- |
| pilot s'intègre nativement dans Claude Code, Cursor et tout agent MCP-compatible. Ton agent IA peut initialiser le projet, switcher d'environnement, déployer sur GCP et monitorer la production sans quitter le chat. Du git init au cloud provider, en une journée, sans expertise DevOps préalable. |

pilot est l'outil que les développeurs qui codent avec l'IA attendaient : non pas un outil qui génère du code, mais un outil qui rend l'infrastructure aussi simple que le code lui-même.