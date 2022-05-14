sgzip
=====
This is an experimental implementation of gzip that allows seeking in the compressed file. In normal gzip files that can only be achieved by decompressing from the start and discarding all data until the selected offset. This gzip implementation works around this by creating a special metadata file that maps uncompressed blocks to compressed blocks allowing it to only read the compressed blocks required.

Due to necessity of being able to start decompression from any block the dictionary is reset after every block. This somewhat negatively effects compression ratio but is a necessary tradeof in our use case. The gzip files created by this library are valid normal gzip files and can be decompressed by any other gzip implementation.

# Warning
This library was purpose build for the `rclone` compression backend. If you are looking for a multithreaded golang gzip implementation you should be using [klauspost/pgzip](https://github.com/klauspost/pgzip) which is the base for this library.


# License
This contains large portions of code from the go repository - see GO_LICENSE for more information. The changes are released under MIT License. See LICENSE for more information.
