# LICENSING — double licence AGPL-3.0 + commerciale

`ds-agent` est distribué sous **deux licences au choix** (*dual licensing*) :

| | Licence | Référence |
|---|---|---|
| 🆓 **Open source** | GNU AGPL v3.0 *only* | [`LICENSE`](LICENSE) |
| 💼 **Commerciale** | contrat sur mesure | §3 ci-dessous |

Identifiant SPDX : `AGPL-3.0-only OR LicenseRef-Commercial`.

## 1. Pourquoi l'AGPL

L'AGPL est un copyleft fort conçu pour les logiciels utilisés en réseau : toute version
modifiée, **y compris proposée comme service en ligne**, doit rester libre et publiée.

En pratique : si vous intégrez `ds-agent` (ou une version modifiée) dans un produit ou un
service accessible par réseau, vous devez publier la source complète de votre version sous
AGPL, modifications comprises.

## 2. Quand la licence commerciale est nécessaire

| Usage | AGPL suffit ? |
|---|---|
| Essai, recherche, usage personnel | ✅ |
| Projet open source sous licence AGPL-compatible | ✅ |
| Outil interne utilisé par vos salariés, sans accès de tiers | ✅ |
| CLI vendue à des clients, code fermé | ❌ → commerciale |
| Service SaaS exposé à des utilisateurs | ⚠️ AGPL impose la publication, sinon commerciale |
| Produit embarqué / OEM / marque blanche | ❌ → commerciale |
| Besoin d'une garantie, d'un SLA ou d'une indemnisation IP | ❌ → commerciale |

> Aide à la décision, **pas un avis juridique**. En cas de doute sur votre conformité,
> consultez un juriste.

## 3. Licence commerciale — cadre

Ce qui suit résume le contrat ; seul le contrat signé fait foi.

**Objet.** Droit non exclusif, non transférable, mondial d'utiliser, modifier, compiler,
distribuer et exploiter `ds-agent` et ses dérivés **sans les obligations de l'AGPL**,
notamment sans publier votre code source.

**Périmètre couvert.** Usage interne (build, exécution, pipelines) ; intégration dans un
produit ou service propriétaire, distribué ou hébergé ; redistribution OEM / marque blanche
dans la limite du volume négocié ; mention factuelle « propulsé par ds-agent ».

**Hors périmètre.** Revente du logiciel comme produit autonome identique, sous-licence à un
tiers concurrent, retrait des mentions de copyright.

**Articulation avec l'AGPL.** Le code reste publié sous AGPL : la licence commerciale ne
révoque pas les droits que l'AGPL vous confère, elle les remplace à votre convenance.

**Offres.** Community (AGPL seule, support communautaire) · Commercial (usage propriétaire,
support e-mail) · Commercial + SLA (réponse sous 2 jours ouvrés) · Enterprise (SLA
contractuel, indemnisation IP, audit de conformité, facturation annuelle).

**Tarification et durée.** Par siège, par site, par déploiement ou pourcentage du chiffre
d'affaires, fixées au devis. Sans devis signé, aucune licence commerciale n'est accordée.

**Données.** L'agent utilise par défaut une API tierce (DeepSeek) : prompts et extraits de
code quittent la machine. Le Concédant ne collecte ni ne conserve de données d'usage. La
conformité de la clé API et du traitement des données relève de votre responsabilité (voir
[SECURITY.md](../SECURITY.md)).

**Garanties.** Fourniture « en l'état », sans garantie d'absence de défaut. Pas de
responsabilité pour dommages indirects, pertes d'exploitation ou destruction de données —
l'agent exécute des commandes shell et modifie des fichiers. Garantie de conformité et
indemnisation plafonnées au prix payé sur les douze derniers mois.

**Fin.** Non-paiement ou violation : résiliation après mise en demeure restée sans effet
30 jours ; vous cessez alors l'exploitation du logiciel propriétaire et le retirez des
produits distribués.

**Contact.** Issue GitHub intitulée « Licence commerciale » (sans détail sensible) ou
l'adresse indiquée sur le profil du mainteneur.

## 4. Contribuer

Sauf mention contraire, toute contribution (issue, PR, patch) est acceptée sous la double
licence du projet : AGPL-3.0 + commerciale, avec le droit pour le mainteneur de la
redistribuer sous l'une ou l'autre. Cela rend le modèle *dual licensing* viable. Si votre
employeur détient des droits sur votre contribution, assurez-vous qu'il autorise cette
cession non exclusive.

## 5. Questions fréquentes

**L'AGPL s'applique-t-elle si j'utilise le binaire en local ?** Non : exécuter localement ne
déclenche aucune obligation. Modifier ou redistribuer, oui.

**Puis-je coder un logiciel propriétaire avec `ds-agent` ?** Oui : le code que vous produisez
vous appartient. L'AGPL porte sur `ds-agent`, pas sur ses sorties.

**Et les dépendances ?** `github.com/mark3labs/mcp-go` (MIT) et les modules indirects sont
compatibles AGPL ; la licence commerciale ne les affecte pas.

---

Copyright (C) 2025 les contributeurs de `ds-agent`.
