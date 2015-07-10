# octogram
#### Fast solver of the octogram puzzle from New Zealand

A friend of mine lent me a puzzle he once bought in New Zealand. Its called Octogram Puzzle, and a very similar puzzle is known as Draught Puzzle. Its goal is to put a number of differently shaped pieces into a square board, while the boards size is 8×8 squares and the pieces are variants of connected squares.

It takes quite some time to find a solution, and I was so annoyed that you just have to try around and almost can’t solve it in a systematic way, that I started to think about how you could efficiently find all possible solutions of the puzzle with a computer algorithm.

The key to a fast algorithm seems to be to detect „dead ends“ of the search path tree as early as possible, that is, partly filled boards that don’t lead to any solution, so that you can skip large amounts of search paths. A typical case are small enclosures (for instance a single square), that none of the left pieces fit into.

I ended up with a modified flood fill algorithm, that starts at a corner, fills that corner with each of the pieces, and then recursively fills all the border squares of the piece with other pieces and so on. This is done by keeping a queue of squares that must be filled next. This way, if putting a piece on the board creates an enclosure, in which none of the left pieces fits into, a part of the enclosure will be border squares of the just added piece and will be queued to be filled. So the enclosure is expected to be detected relatively early, depending on the queue size.

Since every solution exists in 8 equivalent orientations (4 rotations x 2 transpositions) I also added conditions for the pieces filling the four corner squares in order to skip all equivalent orientations.

My single-threaded C++ implementation found all possible solutions (16146) in 22 minutes on a 2.6 GHz CPU, compiled with g++ 4.4.3 and -O3.

This is my implementation in Go, that can find solutions concurrently (see -d option).

I am very interested in even faster algorithms or hints how I can further improve my algorithm, so please comment!

<a rel="license" href="http://creativecommons.org/licenses/by-nc-sa/4.0/"><img alt="Creative Commons License" style="border-width:0" src="https://i.creativecommons.org/l/by-nc-sa/4.0/88x31.png" /></a><br />This work is licensed under a <a rel="license" href="http://creativecommons.org/licenses/by-nc-sa/4.0/">Creative Commons Attribution-NonCommercial-ShareAlike 4.0 International License</a>.
