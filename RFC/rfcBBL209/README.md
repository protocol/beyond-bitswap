# RFC|BB|L2-08: Handle Arbitrary Block Sizes

* Status: `Brainstorm`

## Abstract

This RFC proposes adding a new type of data exchange to Bitswap for handling blocks of data arbitrarily larger than the 1MiB limit by using the features of common hash functions that allow for pausing and then resuming the hashes of large objects.

## Shortcomings

Bitswap has a maximum block size of 1MiB which means that it cannot transfer all forms of content addressed data. A prominent example of this is Git repos which even though they can be represented as a content addressed IPLD graph cannot necessarily be transferred over Bitswap if any of the objects in the repo exceed 1MiB.

## Description

The major hash functions work by taking some data `D` chunking it up into `n` pieces `P_0...P_n-1` then they modify an internal state `S` by loading pieces into the hash function in some way. This means that there are points in the hash function where we can pause processing and get the state of the hash function so far. Bitswap can utilize this state to effectively break up large blocks into smaller ones.

### Example: Merkle–Damgård constructions like SHA-1 or SHA-2

MD pseudo-code looks roughly like:

```golang
func Hash(D []byte) []byte {
   pieces = getChunks(D)

   var S state
   for i, p := range pieces {
      S = process(S, p) // Call this result S_i
   }

   return finalize(S) // Call this H, the final hash
}
```

From the above we can see that:

1. At any point in the process of hashing D we could stop, say after piece `j`, save the state `S_j` and then resume later
2. We can always calculate the final hash `H` given only `S_j` and all the pieces `P_j+1..P_n-1`

The implication for Bitswap is that if each piece size is not more than 1MiB then we can send the file **backwards** in 1MiB increments. In particular a server can send `(S_n-2, P_n-1)` and the client can use that to compute that `P_n-1` is in fact the last part of the data associated with the final hash `H`. The server can then send `(S_n-3, P_n-2)` and the client can calculate that `P_n-2` is the last block of `S_n-2` and therefore also the second to last block of `H`, and so on.

#### Extension

This scheme requires linearly downloading a file which is quite slow with even modest latencies. However, utilizing a scheme like [RFC|BB|L2 - Speed up Bitswap with GraphSync CID only query](https://github.com/protocol/beyond-bitswap/issues/25) (i.e. downloading metadata manifests up front) we can make this fast/parallelizable

#### Security

In order for this scheme to be secure it must be true that there is only a single pair `(S_i-1, P_i)` that can be produced to match with `S_i`. If the pair must be of the form `(S_i-1, P_malicious)` then this is certainly true since otherwise one could create a collision on the overall hash function. However, given that there are two parameters to vary it seems possible this could be computationally easier than finding a collision on the overall hash function.

#### SHA-3

While SHA-3 is not a Merkle–Damgård construction it follows the same psuedocode structure above

### Example: Tree constructions like Blake3, Kangaroo-Twelve, or ParallelHash

In tree constructions we are not restricted to downloading the file backwards and can instead download the parts of the file the we are looking for, which includes downloading the file forwards for sequential streaming.

There is detail about how to do this for Blake3 in the [Blake3 paper](https://github.com/BLAKE3-team/BLAKE3-specs/blob/master/blake3.pdf) section 6.4, Verified Streaming

### Implementation Plan

#### Bitswap changes

* When a server responds to a request for a block if the block is too large then instead send a traversal order list of the block as defined by the particular hash function used (e.g. linear and backwards for SHA-1,2,3)
* Large Manifests
  * If the list is more than 1MiB long then only send the first 1MiB along with an indicator that the manifest is not complete
  * When the client is ready to process more of the manifest then it can send a request WANT_LARGE_BLOCK_MANIFEST containing the multihash of the entire large block and the last hash in the manifest
* When requesting subblocks send requests as `(full block multihash, start index, end index)`
  * process subblock responses separately from full block responses verifying the results as they come in
* As in [RFC|BB|L2 - Speed up Bitswap with GraphSync CID only query](https://github.com/protocol/beyond-bitswap/issues/25) specify how much trust goes into a given manifest, examples include
  * download at most 20 unverified blocks at a time from a given manifest
  * grow trust geometrically (e.g. 10 blocks, then if those are good 20, 40, ...)

#### Datastore

* Servers should cache/store a particular chunking for the traversal that is defined by the implementation for the particular hash function (e.g. 256 KiB segments for SHA-2)
  * Once clients receive the full block they should process it and store the chunking, reusing the work from validating the block
* Clients and servers should have a way of aliasing large blocks as a concatenated set of smaller blocks
* Need to quarantine subblocks until the full block is verified as in [RFC|BB|L2 - Speed up Bitswap with GraphSync CID only query](https://github.com/protocol/beyond-bitswap/issues/25)

#### Hash function support

* Add support for SHA-1/2 (should be very close to the same)
* Make it possible for people to register new hash functions locally, but some should be built into the protocol

## Evaluation Plan

* IPFS file transfer benchmarks as in [RFC|BB|L2 - Speed up Bitswap with GraphSync CID only query](https://github.com/protocol/beyond-bitswap/issues/25)

## Prior Work

* This proposal is almost identical to the one @Stebalien proposed [here](https://discuss.ipfs.io/t/git-on-ipfs-links-and-references/730/6)
* Utilizes overlapping principles with [RFC|BB|L2 - Speed up Bitswap with GraphSync CID only query](https://github.com/protocol/beyond-bitswap/issues/25)

### Alternatives

An alternative way to deal with this problem would be if there was a succinct and efficient cryptographic proof that could be submitted that showed the equivalence of two different DAG structures under some constraints. For example, showing that a single large block with a SHA-2 hash is the equivalent to a tree where the concatenated leaf nodes give the single large block.

### References

This was largely taken from [this draft](https://hackmd.io/@adin/sha256-dag)

## Results

## Future Work
