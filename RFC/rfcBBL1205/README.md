# RFC|BB|L1/2-05: Use of super nodes and decentralized trackers
* Status: `brainstorm`

### Abstract

This RFC proposes the classification of nodes in different types according to their capabilities, and the use of side-channel information to track and discover content in the network. We propose the use of decentralized trackers (with good knowledge of where content is stored in the network and a discovery service for "magnet links"), and supernodes (nodes with high bandwidth and low latency which can significantly improve the transmission of content). Thus, nodes can follow different strategies to speed-up the discovery and transmission by "looking-up" content in decentralized trackers and delegating the download of content to near supernodes.

This RFC will leverage the "high-quality" infrastructure deployed by entities such as Pinata, Infura or PL. We need to acknowledge the existence of these "high-class" nodes and leverage them to improve the performance of the network.

### Description

Introduce in the network the concept of supernodes and decentralize trackers.

-   Supernodes are nodes with high bandwidth, low latency and a good knowledge of where to discover content in the network. Regular nodes would prioritize connection to super nodes as they will speed their file-sharing process. This could be seen as "decentralized gateways" in the network.

-   Decentralized trackers: Similar concept to the one of the "Hydra Boost". These nodes are passive nodes responsible for random walking the network for content and listening to WANT messages or any other additional announcement of metadata exchange devised for content discovery.

Nodes would point decentralize trackers to speed their content discovery and supernodes (if one of them end up being the provider of the content) to increase the transmission.

We could envision the use of side channel identifiers for content discovery, equivalent to "magnet links", which instead of pointing to the specific content, it points to the decentralized tracker that can serve your request better. These mangent links should be "alive" and update with the status of the network. Thus, we could have:

-   `/ipfs/<cid>` identifiers directly pointing to content.

-   `/iptrack/<tracker_id>`: Points to the tracker that may node where to find the content.

-   Additionally, the tracker could answer with `[/p2p/Qm.., /p2p/Qm..]` with a list of supernodes that would lead to a faster download of the content.

### Prior Work

This is similar or can be linked to the [RFC: Side Channels aka DHT-free Content Resolution from this document.](https://docs.google.com/document/d/1QKso-VwYv9jLxTN7WP_RAArrOLCZwjqdjBKQA2wa3VY/edit#)

- [`2Fast: Collaborative downloads in P2P networks`](http://www.st.ewi.tudelft.nl/iosup/2fast06ieeep2p.pdf) proposes the idea of delegating the download of content to a group of nodes. We could consider the implementation of a "grouping scheme" for supernodes in which a node can request a group of supernodes to help him/her download content. This same grouping strategy could be considered for plain nodes as an independent RFC (combination of ideas presented in [RFCBBL207](./rfcBBL207) and [RFCBBL208](./rfcBBL208)).
- [`Exploiting Heterogeneity in Peer-to-Peer Systems Using Gradient Topologies`](https://www.semanticscholar.org/paper/Exploiting-heterogeneity-in-peer-to-peer-systems-Sacha/e981b0e14808a9c66c42295c9f73347f665889b3) is a Ph.D. dissertation that generalises the concept of super-peer into gradient topology networks where eachs peer position is determined by a continuous utility function.
- [`Optimizing cluster formation in super-peer networks via local incentive design`](https://www.semanticscholar.org/paper/Optimizing-cluster-formation-in-super-peer-networks-Kurve-Griffin/ddec940d7582ace67f841b3eeabb3f2ad3867771) proposes an incentive mechanism for optimizing load distribution among a set of super-nodes based on semantic similarities between content interests. The authors propose a game theoretic framework that allows stable Nash equilibria to exist which guarantees the convergence to an optimal, stable peer to super-peer assignment. The approach is optimal in the sense that the average content query resolution time is minimized.


### Implementation Plan

-   [ ] Implementation of super-nodes and the download delegation protocol.

-   [ ] Implementation of decentralized trackers and magnet links protocol.

-   [ ] Evaluation of different discovery and transmission strategies using this network hierarchy.

-   [ ] Group of supernodes strategy.

### Evaluation Plan

-   [The IPFS File Transfer benchmarks.](https://docs.google.com/document/d/1LYs3WDCwpkrBdfrnB_LE0xsxdMCIhXdCchIkbzZc8OE/edit#heading=h.nxkc23tlbqhl)

### Impact

