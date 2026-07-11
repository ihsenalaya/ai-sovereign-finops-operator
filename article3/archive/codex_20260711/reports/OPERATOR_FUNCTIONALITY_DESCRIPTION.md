# Description de l'operateur et de ses fonctionnalites

Ce document donne une vue d'ensemble claire de l'operateur actuel afin de
servir de base au travail scientifique GOV-AR.

## Role general

L'AI Sovereign FinOps Operator est un plan de controle Kubernetes pour les
appels IA d'entreprise. Il ne remplace pas la gateway de donnees; il lit la
telemetrie, applique des politiques declaratives, consolide les couts et peut
actuer certaines decisions de gouvernance.

## Fonctionnalites principales

### 1. Gouvernance des couts

L'operateur attribue les tokens et les couts aux namespaces, equipes,
applications, modeles et fournisseurs. Cette fonction permet de savoir qui
consomme quoi, d'exposer ces chiffres en metriques et de construire des
rapports exploitables.

### 2. Budgets et phases budgetaires

L'operateur suit les depenses par cible de gouvernance et les compare a une
politique budgetaire. Il distingue des phases comme `ok`, `warning`,
`critical`, et `hardLimit`, afin de faire remonter les alertes ou d'activer des
actions de repli.

### 3. Fallback budgetaire

Lorsqu'un budget est sous pression, l'operateur peut recommander ou activer un
mode de degradation vers un modele managé moins cher. Cette fonctionnalite
cherche a conserver le service tout en reduisant la depense.

### 4. Gouvernance de souverainete

L'operateur verifie les contraintes de residence des donnees et d'usage des
fournisseurs externes. Il signale les violations potentielles et, selon le mode
d'enforcement, peut se limiter au reporting ou agir sur le chemin de routage.

### 5. Recommendation engine

L'operateur produit des recommandations chiffrees qui tiennent compte du cout,
du contexte de gouvernance et de la conformite. Une recommandation economique
n'est pas proposee si elle viole une contrainte dure de souverainete.

### 6. Break-even managed vs self-hosted

L'operateur compare les couts d'une API managée avec ceux d'une alternative
auto-hebergee. L'objectif est d'indiquer quand un volume d'usage rend une
strategie self-hosted economiquement interessante.

### 7. Reporting consolide

Les resultats de cout, de souverainete et de recommandations sont consolides
dans des CRDs et des ConfigMaps. Cela fournit un dossier lisible pour les
equipes plateforme, FinOps, securite et audit.

### 8. Observabilite Prometheus et Grafana

L'operateur expose des metriques `ai_finops_*` qui couvrent couts, budgets,
findings, recommandations, quality gates et enforcement. Elles sont destinees a
l'exploitation continue et au diagnostic des politiques.

### 9. Quality gates applicatifs

Avant certains changements de modele, l'operateur peut comparer un modele
candidat a une reference sur un jeu de donnees ou des observations de terrain.
Cette fonctionnalite reduit le risque de deploiement d'un modele moins cher
mais degrade pour l'application.

### 10. Workflow de changement gouverne

Le couple `AIRoutingPolicy -> AIChangeRequest` cree un chemin de changement
auditable avec approbation humaine, etat, verdict et signatures selon la
configuration. Cela structure les modifications de routage sensibles.

### 11. Overrides de routage

Les `AIRouteOverride` permettent un override cible et explicite sur une route
gateway. C'est utile pour des mesures ponctuelles, des incidents ou des tests
encadres.

### 12. Catalogue de providers et de modeles

L'operateur maintient un catalogue de fournisseurs et modeles connus avec prix,
zone et caracteristiques. Cela permet de demarrer vite, de resoudre des modeles
vers leurs providers et d'eviter des zones aveugles dans la facturation.

### 13. Detection Shadow AI par eBPF

Avec Tetragon, l'operateur peut voir des egress qui contournent la gateway.
Cette fonctionnalite complete la gouvernance en couvrant les appels non visibles
par la seule telemetrie applicative.

### 14. Gouvernance confidentielle et attestation

Une seconde famille de CRDs couvre l'attestation et le placement verifiable sur
des noeuds confidentiels. Cela sert les workloads qui exigent une preuve de
plateforme de confiance avant execution ou liberation de cle.

## Pourquoi c'est important pour GOV-AR

L'operateur apporte deja :

- les objets de gouvernance ;
- la logique de reconciliation ;
- les surfaces de metriques ;
- les points d'enforcement ;
- les workflows de changement.

GOV-AR ne repart donc pas de zero. La contribution scientifique consiste a
brancher une nouvelle methode de decision conjointe admission-routage-
reservation sur cette plateforme existante.
