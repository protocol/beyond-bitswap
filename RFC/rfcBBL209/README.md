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

   var St state
   var Sz uint64 = 0
   for i, p := range pieces {
      St = process(S, p) // Call this result S_i
      Sz += size(p)
   }

   return finalize(St, Sz) // Call this H, the final hash
}
```

From the above we can see that:

1. At any point in the process of hashing D we could stop, say after piece `j`, save the state `S_j` and then resume later
2. We can always calculate the final hash `H` given only `S_j` and all the pieces `P_j+1..P_n-1`

The implication for Bitswap is that if each piece size is not more than 1MiB then we can send the file **backwards** in 1MiB increments. In particular a server can send `(S_n-2, P_n-1)` and the client can use that to compute that `P_n-1` is in fact the last part of the data associated with the final hash `H`. The server can then send `(S_n-3, P_n-2)` and the client can calculate that `P_n-2` is the last block of `S_n-2` and therefore also the second to last block of `H`, and so on.

N.B. A helpful analogy might be linked lists.
A Merkle–Damgård hash is a (highly imbalanced) Merkle tree in the same sense that a linked list is a (highly imbalanced) tree.
The only caveat is that each node is a "freestart" hash that begins with the parent hash rather than the normal fixed initial state, hashing the parent hash like data.

#### Statelessness

By finalizing the intermediate hashes, we can make "genuine" requests for the prefixes of data.
This makes the reverse-streaming protocol a bit less of a special case.

For example, imagine if whenever a large file was added, every n MiB prefix was also indecently added.
Then imagine that the response of the first request of the tail of the file is the remainder:
instead of being a whole 1 MiB, it is just enough to take us back to the MiB boundary.
Now, every subsequent request is a for the tail of one of those separately-added prefix files.

Of course in practice, we would not want to do something naive as separately storing prefixes, wasting quadratic space.
But maybe storing all the (finalized) MiB-boundary hashes would be OK (merely linear space).

#### Extension

This scheme requires linearly downloading a file which is quite slow with even modest latencies. However, utilizing a scheme like [RFC|BB|L2 - Speed up Bitswap with GraphSync CID only query](https://github.com/protocol/beyond-bitswap/issues/25) (i.e. downloading metadata manifests up front) we can make this fast/parallelizable

#### Security

In order for this scheme to be secure it must be true that there is only a single pair `(S_i-1, P_i)` that can be produced to match with `S_i`. If the pair must be of the form `(S_i-1, P_malicious)` then this is certainly true since otherwise one could create a collision on the overall hash function. However, given that there are two parameters to vary it seems possible this could be computationally easier than finding a collision on the overall hash function.

#### SHA-3

While SHA-3 is not a Merkle–Damgård construction it follows the same psuedocode structure above

### Example: Tree constructions like Blake3, Kangaroo-Twelve, or ParallelHash

In tree constructions we are not restricted to downloading the file backwards and can instead download the parts of the file the we are looking for, which includes downloading the file forwards for sequential streaming.

There is detail about how to do this for Blake3 in the [Blake3 paper](https://github.com/BLAKE3-team/BLAKE3-specs/blob/master/blake3.pdf) section 6.4, Verified Streaming.freestart
Note that the merkle tree is more legitimate in this case, because there is nothing like the "freestart" caveat that may weaken the security of tail blocks for Merkle–Damgård construction hashes.
Also note that because of the lack of a free start their is less associativity:
whereas the chunking size doesn't matter for SHA-1 construction, it does for Blake3.
However, there is still finalization in that only the root note is hashed with the `ROOT` flag set.

### Implementation Plan

#### Bitswap changes

##### Merkle–Damgård

Let `CHUNK_SIZE` be some constant such that no response overflows the bitswap limit.

* When a server responds to a request for a block, if the block is too large then instead send the final block which will hash to the requested hash: `(S_n-1, P_n, total_size)`.
  The length of a the block be such that the remainder is can be split into an exact number whole chunks:
  ```golang
  (total_size - size(P_n)) % CHUNK_SIZE == 0
  ```

* The client verifies the final hash:
  ```golang
  S_n == finalize(process(S_n-1, P_n), total_size)
  ```

* The client can request the previous chunk with a finalized hash computed by:
  ```golang
  S_n-1 = finalize(S_n-1, total_size - size(P_n))
  ```
  Unlike with the original request, the client now has an expected size of the remainder which it can verify against the server's response.

##### Blake3

* When a server responds to a request for a block, if the block is too large then instead send the final chunk and the size.

* The client can verify the block.

* Using the size, calculate what the sizes of the children should be, or whether in fact the block is a parent block.
  The formula is given in the paper.

* If the block is a parent block the client may request the children:

  * Simply take the first or second 32 bytes to have a new hash.

  * However, use a different multihash to account for `ROOT=false`.

  * Verify the size given in the response against the calculated subtree size.

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

## Future work

To trade trustless for less sequentiality, utilize [RFC|BB|L2 - Speed up Bitswap with GraphSync CID only query](https://github.com/protocol/beyond-bitswap/issues/25) to requesting the children of yet-unfetched chunks.

### Alternatives

An alternative way to deal with this problem would be if there was a succinct and efficient cryptographic proof that could be submitted that showed the equivalence of two different DAG structures under some constraints. For example, showing that a single large block with a SHA-2 hash is the equivalent to a tree where the concatenated leaf nodes give the single large block.

### References

This was largely taken from [this draft](https://hackmd.io/@adin/sha256-dag)

## Results

## Future Work
