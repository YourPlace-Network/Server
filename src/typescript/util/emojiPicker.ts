import { Picker } from "emoji-picker-element";

let activeEmojiPopup: HTMLElement | null = null;

export function createEmojiPicker(onSelect: (emoji: string) => void): HTMLElement {
    const picker = new Picker();
    picker.addEventListener("emoji-click", (event: any) => {
        const emoji = event.detail.unicode;
        onSelect(emoji);
    });
    return picker as unknown as HTMLElement;
}

export function showEmojiPicker(anchorElement: HTMLElement, onSelect: (emoji: string) => void): void {
    if (activeEmojiPopup) {
        activeEmojiPopup.remove();
        activeEmojiPopup = null;
    }
    const popup = document.createElement("div");
    popup.classList.add("emojiPickerPopup");
    const picker = new Picker();
    picker.addEventListener("emoji-click", (event: any) => {
        const emoji = event.detail.unicode;
        onSelect(emoji);
        popup.remove();
        activeEmojiPopup = null;
    });
    popup.appendChild(picker);
    const rect = anchorElement.getBoundingClientRect();
    const scrollX = window.scrollX || window.pageXOffset;
    const scrollY = window.scrollY || window.pageYOffset;
    popup.style.position = "absolute";
    popup.style.left = `${rect.left + scrollX}px`;
    popup.style.top = `${rect.bottom + scrollY + 5}px`;
    popup.style.zIndex = "10000";
    document.body.appendChild(popup);
    activeEmojiPopup = popup;
    popup.addEventListener("mousedown", (e: MouseEvent) => {
        e.stopPropagation();
    });
    popup.addEventListener("click", (e: MouseEvent) => {
        e.stopPropagation();
    });
    const closeOnOutsideClick = (e: MouseEvent) => {
        const path = e.composedPath();
        const isInsidePopup = path.some(el => el === popup);
        if (!isInsidePopup) {
            popup.remove();
            activeEmojiPopup = null;
            document.removeEventListener("click", closeOnOutsideClick, true);
        }
    };
    setTimeout(() => {
        document.addEventListener("click", closeOnOutsideClick, true);
    }, 100);
}

export function closeEmojiPicker(): void {
    if (activeEmojiPopup) {
        activeEmojiPopup.remove();
        activeEmojiPopup = null;
    }
}

export function setupTinyMCEEmojiButton(editor: any): void {
    editor.ui.registry.addButton("emojipicker", {
        icon: "emoji",
        tooltip: "Insert Emoji",
        onAction: () => {
            const toolbar = editor.getContainer().querySelector(".tox-toolbar__primary") as HTMLElement;
            const anchor = toolbar || editor.getContainer();
            showEmojiPicker(anchor, (emoji: string) => {
                editor.insertContent(emoji);
            });
        }
    });
    editor.ui.registry.addIcon("emoji", '<svg width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="M12 2C6.47 2 2 6.47 2 12s4.47 10 10 10 10-4.47 10-10S17.53 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm3.5-9c.83 0 1.5-.67 1.5-1.5S16.33 8 15.5 8 14 8.67 14 9.5s.67 1.5 1.5 1.5zm-7 0c.83 0 1.5-.67 1.5-1.5S9.33 8 8.5 8 7 8.67 7 9.5 7.67 11 8.5 11zm3.5 6.5c2.33 0 4.31-1.46 5.11-3.5H6.89c.8 2.04 2.78 3.5 5.11 3.5z"/></svg>');
}
