The command line flag `-c` accepts a 32-byte array that is hex encoded as a string. This string is used to set the crypto 
seed, which allows multiple instances to share the same cryptographic state, enabling horizontal scaling.

Running as a gateway by setting the `-m` flag to `` will require a PostgreSQL wire protocol compatible database. Documentation will focus around CockroachDB
as it is open-source, well supported, scales elastically and is made for cloud native environments.

Sandboxing of the binary is recommended, as it is a network facing application. The binary should be run as a non-root user.
** resources/WindowsSandboxConfig.wsb - Windows Sandbox configuration file
** sandboxie - Windows sandboxing tool
** resources/YourPlace.apparmor - AppArmor profile for Linux
** "sandbox-exec -f resources/YourPlace.sb <InstallDir>/YourPlace" - macOS sandboxing tool
** Entitlements.plist - macOS entitlements file + signed App Bundle - macOS sandboxing tool (needs Apple Develper ID)
