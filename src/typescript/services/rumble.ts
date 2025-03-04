export function RumbleEmbed(inputStr: string, height: number, width: number): HTMLIFrameElement | null {
    const rumbleRegex = /https?:\/\/(?:www\.)?rumble\.com\/(?:v|embed)\/([a-zA-Z0-9_-]+)/g;
    let match;
    while ((match = rumbleRegex.exec(inputStr)) !== null) {
        if (match[1]) {
            const videoID = match[1];
            const iframe = document.createElement("iframe");
            iframe.width = width.toString();
            iframe.height = height.toString();
            iframe.src = `https://rumble.com/embed/${videoID}/`;
            iframe.setAttribute("frameborder", "0");
            iframe.setAttribute("allow", "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture");
            iframe.setAttribute("allowfullscreen", "");
            return iframe;  // Return the first matching iframe
        }
    }
    return null;
}

// Example usage
/*const inputStr = "Check out this amazing video: https://rumble.com/vkrd7x-amazing-video.html It's awesome!";
const iframeElement = createRumbleEmbed(inputStr);
if (iframeElement) {
    document.body.appendChild(iframeElement);
}*/

