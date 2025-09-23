export function GetPageRoute(): string {
    let page = window.location.pathname;
    const segments: string[] = page.split("/");
    return segments[1] || "";
}