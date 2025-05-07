# Helper Architecture

The “helper” is a binary embedded in the YourPlace server, that is dropped on the host OS and run independently of the server itself. The helper performs administrative actions on behalf of the server, to allow the server to run as a low-privledged user. The server communicates with the helper over IPC, using mechanisms dependent on the host OS. The helper starts on computer boot up, and will restart via a scheduled task/systemd task if its process is ever killed.

The server communicates with the helper via a request / response architecture and can send a single string back and forth.

### Commands

* install
* uninstall
* uninstall -keepUploads
* uninstall -keepBlockchain
* uninstall -keepUploads -keepBlockchain
* version
* restart

