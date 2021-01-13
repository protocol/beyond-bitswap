# RFC|BB|L2-10: UnixFS files identified using hash of the full content

* Status: `Brainstorm`

## Abstract

This RFC proposes that for UnixFS files we allow for downloading data using a CID corresponding to the hash of the entire file instead of just the CID of the particular UnixFS DAG (tree width, chunking, internal node hash function, etc.).

Note: This is really more about IPFS than Bitswap, but it's close by and dependent on another RFC.

## Shortcomings

There exists a large quantity of content on the internet that is already content addressable and yet not downloadable via IPFS and Bitswap. For example, many binaries, videos, archives, etc. that are distributed today have their SHA-256 listed along side them so that users can run `sha2sum file` and compare the output with what they were expecting. When these files are added to IPFS they can be added as: a) An application-specific DAG format for files (such as UnixFSv1) which are identified by a DAG root CID which is different from a CID of the multihash of the file data itself b) a single large raw block which cannot be processed by Bitswap.

Additionally, for users using application specific DAGs with some degree of flexibility to them (e.g. UnixFS where there are multiple chunking strategies) two users who import the same data could end up with different CIDs for that data.

## Description

Utilizing the results of [RFCBBL209](../rfcBBL209/README.md) we can download arbitrarily sized raw blocks. We allow UnixFS files that have raw leaves to be stored internally as they are now but also aliased as a single virtual block.

## Implementation plan

* Implement [RFCBBL209](../rfcBBL209/README.md)
* Add an option when doing `ipfs add` that creates a second aliased block in a segregated blockstore
* Add the second blockstore to the provider queue

## Impact

This scheme allows a given version of IPFS to have a canonical hash for files (e.g. SHA256 of the file data itself), which allows for independent chunking schemes, and by supporting the advertising/referencing of one or more common file hash schemes allow people to find some hash on a random website and check to see if it's discoverable in IPFS.

There are also some larger ecosystem wide impacts to consider here, including:

1. There's a lot of confusion around UnixFS CIDs not being derivable from SHA256 of a file, this approach may either tremendously help or cause even more confusion (especially as we move people from UnixFS to IPLD). An example [thread](https://discuss.ipfs.io/t/cid-concept-is-broken/9733) about this
2. Storage overhead for multiple "views" on the same data and extra checking + advertising of the data
3. Are there any deduplication use case issues we could run into here based on users not downloading data that was chunked as the data creator did it, but instead based on how they want to chunk it (or likely the default chunker)
4. File identified using hash of the full content enables [validation of HTTP gateway responses](https://github.com/ipfs/in-web-browsers/issues/128) without running full IPFS stack, which allows for:
   - user agents such as web browsers: display integrity indicator when HTTP response matched the CID from the request 
   - IoT devices: downloading firmware updates over HTTPS without the need for trusting a gateway or a CA

## Evaluation Plan

TBD

## Prior Work

## Results

## Future Work
