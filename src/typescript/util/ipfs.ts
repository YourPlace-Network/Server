import {CID} from "multiformats/cid";
import {IsValidIpfsCid} from "./security";
import {HttpGetJson} from "./network";

/*async function ipfsInit() { // Initialize the Helia/kubo-rpc node and in-browser blockstore and file system
    if (window.kuboClient == null) {
        const [status, data] = await HttpGetJson("/settings/ipfs/port");
        const port = parseInt(data["port"]);
        window.kuboClient = create({host: "127.0.0.1", port: port, protocol: "http", timeout: 10000});
    }
}*
ipfsInit();*/

/* --- IPFS Primitives --- */
/*export async function ipfsUploadFile(_file: File): Promise<CID> {
    let cid = null;
    if (window.kuboClient != null) {
        let result = await window.kuboClient.add(_file);
        cid = result.cid;
    }
    return cid!;
}*/
/*export async function ipfsGetFile(cid: CID): Promise<File | null> {
    if (window.kuboClient != null) {
        const file = window.kuboClient.cat(cid.toString());
        return new File(await iterableToBlobArray(file), cid.toString());
    }
    return null;
}*/
/*export async function ipfsGetImageUrl(cid: CID): Promise<string> {
    const file = await ipfsGetFile(cid);
    return URL.createObjectURL(file!);
}*/

/* --- Helper Functions --- */
export function stringToCID(cid: string): CID {
    if (!IsValidIpfsCid(cid)) throw new Error("Invalid CID");
    return CID.parse(cid);
}
async function iterableToBlobArray(asyncIterable: AsyncIterable<Uint8Array>): Promise<BlobPart[]> {
    const blobParts: BlobPart[] = [];
    for await (const chunk of asyncIterable) {
        blobParts.push(chunk);
    }
    return blobParts;
}