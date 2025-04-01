window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import DOMPurify from "dompurify";
import "../../scss/components/modalYesNo.scss"

// HTML Template:  {{template "modalYesNo" .}}

export function ShowModalYesNo(message: string): Promise<boolean> {
    return new Promise((resolve) => {
        document.getElementById("modalYesNoContent")!.textContent = message;
        const yesButton = document.getElementById("modalYesButton") as HTMLButtonElement;
        const noButton = document.getElementById("modalNoButton") as HTMLButtonElement;
        // Create click handlers
        const handleYes = () => {
            cleanup();
            resolve(true);
        }
        const handleNo = () => {
            cleanup();
            resolve(false);
        }
        // Clean up event listeners and hide modal
        const cleanup = () => {
            yesButton.removeEventListener("click", handleYes);
            noButton.removeEventListener("click", handleNo);
            HideModalYesNo();
        };
        // Add event listeners
        yesButton.addEventListener("click", handleYes);
        noButton.addEventListener("click", handleNo);
        // Show modal
        let element = document.getElementById("modalYesNo")!;
        let modal = new window.bootstrap.Modal(element, {
            backdrop: 'static', // Prevent closing when clicking outside
            keyboard: false     // Prevent closing with escape key
        });
        modal.show();
    });
}
export function ShowModalYesNoHTML(message: string) {
    return new Promise((resolve) => {
        document.getElementById("modalYesNoContent")!.innerHTML = DOMPurify.sanitize(message, {USE_PROFILES: {html:true},ADD_ATTR:["target"]});
        // Get reference to buttons
        const yesButton = document.getElementById("modalYesButton") as HTMLButtonElement;
        const noButton = document.getElementById("modalNoButton") as HTMLButtonElement;
        // Create click handlers
        const handleYes = () => {
            cleanup();
            resolve(true);
        };
        const handleNo = () => {
            cleanup();
            resolve(false);
        };
        // Clean up event listeners and hide modal
        const cleanup = () => {
            yesButton.removeEventListener("click", handleYes);
            noButton.removeEventListener("click", handleNo);
            HideModalYesNo();
        };
        // Add event listeners
        yesButton.addEventListener("click", handleYes);
        noButton.addEventListener("click", handleNo);
        // Show modal
        let element = document.getElementById("modalYesNo")!;
        let modal = new window.bootstrap.Modal(element, {
            backdrop: 'static',
            keyboard: false
        });
        modal.show();
    });
}
export function ShowModalYesNoHTMLUnsafe(message: string) {
    return new Promise((resolve) => {
        document.getElementById("modalYesNoContent")!.innerHTML = message;
        // Get reference to buttons
        const yesButton = document.getElementById("modalYesButton") as HTMLButtonElement;
        const noButton = document.getElementById("modalNoButton") as HTMLButtonElement;
        // Create click handlers
        const handleYes = () => {
            cleanup();
            resolve(true);
        };
        const handleNo = () => {
            cleanup();
            resolve(false);
        };
        // Clean up event listeners and hide modal
        const cleanup = () => {
            yesButton.removeEventListener("click", handleYes);
            noButton.removeEventListener("click", handleNo);
            HideModalYesNo();
        };
        // Add event listeners
        yesButton.addEventListener("click", handleYes);
        noButton.addEventListener("click", handleNo);
        // Show modal
        let element = document.getElementById("modalYesNo")!;
        let modal = new window.bootstrap.Modal(element, {
            backdrop: 'static',
            keyboard: false
        });
        modal.show();
    });
}
export function HideModalYesNo() {
    let element = document.getElementById("modalYesNo")!;
    let modal = window.bootstrap.Modal.getInstance(element);
    if (modal) {
        modal.hide();
    }
}