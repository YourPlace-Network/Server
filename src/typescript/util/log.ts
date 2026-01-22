import DOMPurify from "dompurify";

export function LogInfo(message: string) {
    console.log('%c[INFO]%c ' + DOMPurify.sanitize(message), 'color: #0080FF', 'color: #FFFFFF');
}
export function LogError(message: string) {
    console.log('%c[ERROR]%c ' + DOMPurify.sanitize(message), 'color: #FF0000', 'color: #FFFFFF');
}
export function LogDebug(message: string) {
    console.log('%c[DEBUG]%c ' + DOMPurify.sanitize(message), 'color: #0080FF', 'color: #FFFFFF');
}