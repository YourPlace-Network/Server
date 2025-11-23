export function GetPageRoute(): string {
    let page = window.location.pathname;
    const segments: string[] = page.split("/");
    return segments[1] || "";
}
export function IsGatewayMode(): boolean {
    let gatewayMode = document.getElementById("gatewayModeAddPost") as HTMLInputElement;
    if (gatewayMode == null) {
        return false;
    }
    return gatewayMode.value === "false";
}
export function IsMobileDevice(): boolean {
    const userAgent = navigator.userAgent || navigator.vendor || (window as any).opera;
    const mobileRegex = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini|mobile|tablet/i;
    return mobileRegex.test(userAgent.toLowerCase());
}