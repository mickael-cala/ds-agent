# LICENSING — double licence AGPL-3.0 + commerciale

`ds-agent` est distribué sous **deux licences au choix** (modèle *dual licensing*) :

| | Licence | Fichier |
|---|---|---|
| 🆓 **Open source** | GNU AGPL v3.0 *only* | [`LICENSE`](LICENSE) |
| 💼 **Commerciale** | Contrat commercial sur mesure | [`LICENSE-COMMERCIAL.md`](LICENSE-COMMERCIAL.md) |

Identifiant SPDX à utiliser dans les sources et les métadonnées de paquet :

```
AGPL-3.0-only OR LicenseRef-Commercial
```

## Pourquoi l'AGPL ?

L'AGPL est une licence copyleft forte, conçue pour les logiciels utilisés en réseau.
Elle garantit que toute version modifiée, **y compris lorsqu'elle est proposée comme
service en ligne**, reste libre et que son code source est accessible à ses utilisateurs.

Concrètement, si vous intégrez `ds-agent` (ou une version modifiée) dans un produit ou un
service accessible par réseau, vous devez publier la source complète de votre version
sous AGPL — y compris vos modifications.

## Quand la licence commerciale est nécessaire

La licence commerciale est destinée à celles et ceux qui **ne peuvent pas ou ne veulent pas**
respecter les obligations de l'AGPL :

- intégration dans un produit **propriétaire** distribué ou hébergé (SaaS) ;
- revente, marque blanche, OEM ;
- besoin d'une **garantie**, d'un support SLA ou d'une indemnisation (IP indemnity) ;
- conditions de conformité internes interdisant le copyleft fort ;
- redistribution sans obligation de publication des sources.

Elle n'est pas nécessaire pour : usage personnel, usage interne non redistribué ? —
**attention**, l'AGPL couvre aussi l'usage en réseau, voir ci-dessous.

| Usage | AGPL suffit ? |
|---|---|
| Essai, recherche, usage personnel | ✅ |
| Projet open source sous AGPL-compatible | ✅ |
| Outil interne utilisé par vos seuls salariés, sans accès réseau de tiers | ✅ (aucune obligation de publication) |
| CLI vendue à des clients, code fermé | ❌ → commerciale |
| Service SaaS exposé à des utilisateurs | ⚠️ → AGPL impose la publication, sinon commerciale |
| Produit embarqué / OEM | ❌ → commerciale |

> Ce tableau est une aide à la décision, **pas un avis juridique**. En cas de doute sur
> votre conformité, consultez un juriste.

## Contribuer

Sauf mention contraire, toute contribution (issue, pull request, patch) est acceptée sous
la double licence du projet : AGPL-3.0 + commerciale, avec le droit pour le mainteneur de
la redistribuer sous l'une ou l'autre. Cela permet au modèle *dual licensing* de rester
viable (voir [CONTRIBUTING.md](CONTRIBUTING.md)).

Si votre employeur détient des droits sur votre contribution, assurez-vous qu'il autorise
cette cession non exclusive.

## Questions fréquentes

**L'AGPL s'applique-t-elle si j'utilise seulement le binaire `ds-agent` en local ?**
Non, l'exécuter localement ne déclenche aucune obligation de publication. En revanche,
le modifier ou le redistribuer, oui.

**Puis-je utiliser `ds-agent` pour coder un logiciel propriétaire ?**
Oui : les artefacts que l'agent produit vous appartiennent. L'AGPL porte sur `ds-agent`
lui-même, pas sur le code que vous lui faites écrire.

**Et les dépendances ?**
`github.com/mark3labs/mcp-go` (MIT) et les modules indirects sont compatibles avec l'AGPL.
La licence commerciale n'affecte pas ces composants.

**Comment obtenir une licence commerciale ?**
Voir [`LICENSE-COMMERCIAL.md`](LICENSE-COMMERCIAL.md) — contact via une issue GitHub
intitulée « Licence commerciale ».

---

Copyright (C) 2025 les contributeurs de `ds-agent`.
