# Licence commerciale — `ds-agent`

Ce document **n'est pas** le contrat : il en résume le cadre. La licence commerciale
s'obtient par un contrat écrit signé entre vous (ou votre société) et le titulaire des
droits (« le Concédant »).

## 1. Objet

Droit **non exclusif, non transférable, mondial** d'utiliser, modifier, compiler,
distribuer et exploiter `ds-agent` et ses dérivés **sans les obligations de l'AGPL-3.0**,
notamment sans obligation de publier votre code source.

## 2. Périmètre

Le contrat couvre :

- l'usage en interne (build, exécution, intégration dans vos pipelines) ;
- l'intégration dans un produit ou service propriétaire, distribué ou hébergé ;
- la redistribution en marque blanche / OEM, dans la limite du volume négocié ;
- les droits d'utilisation des **marques et logos** du projet (uniquement pour la
  mention factuelle « propulsé par ds-agent », sauf accord écrit).

Ne sont **pas** couverts : la revente de `ds-agent` en tant que produit autonome
identique, la sous-licence du logiciel à des tiers en tant que composant concurrent,
et le droit de retirer les mentions de copyright.

## 3. Ce qui reste AGPL

Le code source reste publié sous AGPL-3.0. La licence commerciale ne révoque pas les
droits que vous confère l'AGPL : elle les **remplace à votre convenance**. Vous n'avez
pas à choisir à l'avance — mais la redistribution du code doit respecter la licence
sous laquelle vous l'avez obtenu.

## 4. Obligations du Concédant

Selon le niveau de contrat :

| Offre | Contenu |
|---|---|
| **Community** | AGPL-3.0 seule, support communautaire via les issues |
| **Commercial** | Droit d'usage propriétaire + support par e-mail |
| **Commercial + SLA** | Réponse sous 2 jours ouvrés, correctifs prioritaires |
| **Enterprise** | SLA contractuel, indemnisation IP, audit de conformité, facturation annuelle |

L'indemnisation IP (garantie que le logiciel ne viole pas de droits de tiers) n'est
fournie qu'avec les offres **Enterprise**, dans les limites du contrat.

## 5. Tarification et durée

La tarification (par siège, par site, par déploiement, ou pourcentage du chiffre
d'affaires) et la durée sont fixées dans le devis. Sans devis signé, aucune licence
commerciale n'est accordée.

## 6. Confidentialité & données

L'agent utilise par défaut une API tierce (DeepSeek) : les prompts et extraits de code
envoyés au modèle quittent la machine. Le Concédant ne collecte ni ne conserve de
données issues de votre usage du binaire. La conformité de la clé API et du traitement
des données relève de votre responsabilité (voir [SECURITY.md](SECURITY.md)).

## 7. Garanties

Le binaire est fourni **« en l'état »**, sans garantie d'absence de défaut.
Le Concédant n'est pas responsable des dommages indirects, pertes d'exploitation ou
destruction de données — l'agent exécute des commandes shell et modifie des fichiers.
La garantie de conformité et l'indemnisation sont limitées, en montant, au prix payé
au cours des douze derniers mois.

## 8. Fin du contrat

En cas de non-paiement ou de violation, la licence se termine après mise en demeure
restée sans effet durant 30 jours. Vous cessez alors toute exploitation du logiciel
propriétaire et devez le retirer des produits distribués.

## 9. Contact

Ouvrez une issue GitHub intitulée **« Licence commerciale »** (sans détail sensible)
ou utilisez l'adresse indiquée sur le profil du mainteneur du dépôt.

---

Ce résumé est fourni à titre informatif et ne constitue pas une offre contractuelle.
Seul le contrat signé fait foi. Lire aussi [LICENSING.md](LICENSING.md).
