//import Hls from "hls";
//TODO: Pass this function the ipfs index and parent div of a video to display

export async function PlayVideo(ipfsIndex: string, parent: HTMLDivElement) {
    const video = document.createElement("video");
    const csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
    let videoURI = "/files/stream/" + ipfsIndex;
    /*if (Hls.isSupported()) { // todo broken
        let hls = new Hls({
            xhrSetup: xhr => {
                xhr.setRequestHeader(
                    "X-CSRF-Token", csrfToken
                )
            }
        });
        hls.loadSource(videoURI);
        hls.attachMedia(<HTMLMediaElement>video);
    }*/
    parent.appendChild(video)
}
