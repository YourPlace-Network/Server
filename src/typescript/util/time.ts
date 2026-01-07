export function formatTimestamp(timestamp: number): string {
    const now = Date.now() / 1000;
    const diff = now - timestamp;
    if (diff < 60) {
        return "just now";
    } else if (diff < 3600) {
        const mins = Math.floor(diff / 60);
        return `${mins}m ago`;
    } else if (diff < 86400) {
        const hours = Math.floor(diff / 3600);
        return `${hours}h ago`;
    } else if (diff < 604800) {
        const days = Math.floor(diff / 86400);
        return `${days}d ago`;
    } else {
        const date = new Date(timestamp * 1000);
        return date.toLocaleDateString();
    }
}
export function Sleep(ms: number) {
    return new Promise( resolve => setTimeout(resolve, ms) );
}