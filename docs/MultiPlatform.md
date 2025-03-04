Most of the multi-platform logic is stored in the src/core/host path.

There are several instances of platform specific code in the code base, so you'll need to search for "runtime.GOOS" function calls to find them. Installing IPFS daemons is one example of this