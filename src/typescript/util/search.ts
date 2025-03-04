import {IsValidAlgoAddress, IsValidIpfsCid} from "./security";
import {HttpGetJson} from "./network";

let responses: [number, any][] = [];
let hashtagCount = 0;
let nfdomainCount = 0;
let algoAddressCount = 0;
let ipfsCount = 0;

export async function searchPlaces(query: string): Promise<[number, any][]>{
    /*responses = await Promise.all([
        HttpGetJson("/search/posts?q=" + query),
        HttpGetJson("/search/profiles?q=" + query),
    ])*/
    /*let splitQuery = query.split(" ");
    for (let searchWord of splitQuery) {
        if (searchWord.startsWith("#")) {
            hashtagCount++;
            searchHashtag(searchWord).then(r => {});
            break;
        } else if (searchWord.endsWith(".algo")) {
            nfdomainCount++;
            searchNFDomain(searchWord).then(r => {});
            break;
        } else if (IsValidAlgoAddress(searchWord)) {
            algoAddressCount++;
            searchAlgoAddress(searchWord).then(r => {});
            break;
        } else if (IsValidIpfsCid(searchWord)) {
            ipfsCount++;
            searchIpfsCid(searchWord).then(r => {});
            break;
        } else {
            searchString(searchWord).then(r => {});
            break;
        }
    }*/
    return responses;
}
async function searchHashtag(hashtag: string) {
    console.log("searching hashtag: ", hashtag);
    let data = {h: hashtag};
    let response = await fetch("/search?" + new URLSearchParams(data));
    let responseJSON = response.json();
    console.log(responseJSON);
    return responseJSON;
}
async function searchAlgoAddress(address: string) {
    console.log("searching algo address: ", address);
}
async function searchNFDomain(address: string) {
    console.log("search nfdomain: ", address);
}
async function searchString(string: string) {
    console.log("search string: ", string);
}
async function searchIpfsCid(cid: string) {
    console.log("search ipfs: ", cid);
}