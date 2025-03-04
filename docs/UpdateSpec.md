# YourPlace Updates

### Goals
* Deliver updates to YourPlace servers within 12 hours of release
* Cryptographically sign and verify updates to ensure integrity
* Use IPFS to deliver updates as a backup mechanism

### YourPlaceUpdate.zip
* Zip file containing the update artifacts:
* * update.json - metadata about the update (version, release date)
* * YourPlace.exe - binary to replace the current version
* * YourPlaceUpdate.sig - signature of SHA3(update.json + YourPlace.exe + update.sql + update.script)
* * update.sql - SQL script to update the database (optional)
* * update.script - Bash/Powershell script to run first (after the zip has been extracted) (optional)

### Process
1. YourPlace servers drop Helper upon install
2. Helper starts as a recurring admin service
3. select 2 random times between 00:00 / 12:00 and 12:01 / 23:59 - wait for those times to arrive then continue the update
4. HTTP Get to yourplace.network/update/latest to get the latest version
5. If the version is newer than the current version, download the update
6. Create or Empty InstallDir/Update
7. HTTP GET to yourplace.network/update?os=win&arch=x64&v=1.0.0 & download file (v = current version of the server) to InstallDir/Update/YourPlaceUpdate.zip
8. Verify the signature of the update
9. Delete InstallDir/Update
10. Extract YourPlaceUpdate.zip to InstallDir/Update
11. Kill YourPlace* processes
12. Run update.script if it exists
13. Run update.sql if it exists
14. Replace YourPlace.exe with the new version
15. Restart YourPlace

