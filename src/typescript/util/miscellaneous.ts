export function GetPageRoute(): string {
    let page = window.location.pathname;
    const segments: string[] = page.split("/");
    return segments[1] || "";
}
export function IsGatewayMode(): boolean {
    const gatewayModeEl = document.getElementById("gatewayMode") as HTMLInputElement;
    const gatewayModeAddPostEl = document.getElementById("gatewayModeAddPost") as HTMLInputElement;
    const gatewayMode = gatewayModeEl?.value || gatewayModeAddPostEl?.value;
    if (gatewayMode == null) {
        return false;
    }
    const hostname = window.location.hostname;
    const isLocalhost = hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]';
    return gatewayMode === "true" && !isLocalhost;
}
export function IsMobileDevice(): boolean {
    const userAgent = navigator.userAgent || navigator.vendor || (window as any).opera;
    const mobileRegex = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini|mobile|tablet/i;
    return mobileRegex.test(userAgent.toLowerCase());
}