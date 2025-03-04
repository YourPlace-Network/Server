window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import DOMPurify from "dompurify";
import "../../scss/components/modalYesNo.scss"

// HTML Template:  {{template "modalYesNo" .}}

export function ShowModalYesNo(message: string) {
    document.getElementById("modalYesNoContent")!.textContent = message;
    let element = document.getElementById("modalDialog")!;
    let modal = new window.bootstrap.Modal(element, {});
    modal.show();
}
export function ShowModalYesNoHTML(message: string) {
    document.getElementById("modalDialogContent")!.innerHTML = DOMPurify.sanitize(message, {USE_PROFILES:{html:true}});
    let element = document.getElementById("modalDialog")!;
    let modal = new window.bootstrap.Modal(element, {});
    modal.show();
}
export function ShowModalYesNoHTMLUnsafe(message: string) {
    document.getElementById("modalDialogContent")!.innerHTML = message;
    let element = document.getElementById("modalDialog")!;
    let modal = new window.bootstrap.Modal(element, {});
    modal.show();
}
export function HideModalYesNo() {
    let element = document.getElementById("modalDialog")!;
    let modal = new window.bootstrap.Modal(element, {});
    modal.hide();
}