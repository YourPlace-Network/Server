import {LogInfo} from "../util/log";

window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import DOMPurify from "dompurify";
import "../../scss/components/modalDialog.scss"

// HTML Template:  {{template "modalDialog" .}}

export function ShowDialogModal(message: string) {
    document.getElementById("modalDialogContent")!.textContent = message;
    let element = document.getElementById("modalDialog")!;
    let modal = new window.bootstrap.Modal(element, {});
    modal.show();
}
export function ShowDialogModalHTML(message: string) {
    document.getElementById("modalDialogContent")!.innerHTML = DOMPurify.sanitize(
        message, {
            USE_PROFILES: {html:true},
            ADD_ATTR: ["target"],
        }
    );
    let element = document.getElementById("modalDialog")!;
    let modal = new window.bootstrap.Modal(element, {});
    modal.show();
}
export function ShowDialogModalHTMLUnsafe(message: string) {
    LogInfo("Potentially Dangerous ShowDialogModalHTMLUnsafe() Used. Ensure that no user-controlled data can reach this function. Usage on this page: " + window.location.toString());
    document.getElementById("modalDialogContent")!.innerHTML = message;
    let element = document.getElementById("modalDialog")!;
    let modal = new window.bootstrap.Modal(element, {});
    modal.show();
}
export function HideDialogModal() {
    let element = document.getElementById("modalDialog")!;
    let modal = new window.bootstrap.Modal(element, {});
    modal.hide();
    document.querySelectorAll(".modal-backdrop").forEach(el => el.remove());
    document.body.classList.remove("modal-open");
}
export function DisableDialogModalOkBtn() {
    (document.getElementsByClassName("yp-modal-btn")[0]! as HTMLButtonElement).style.display = "none";
}
export function DisableDialogModalExit() {
    let modalDialog = document.getElementById("modalDialog")! as HTMLDivElement;
    let attributeBackdrop = document.createAttribute("data-bs-backdrop");
    let attributeKeyboard = document.createAttribute("data-bs-keyboard");
    attributeBackdrop.value = "static";
    attributeKeyboard.value = "false";
    modalDialog.attributes.setNamedItem(attributeBackdrop);
    modalDialog.attributes.setNamedItem(attributeKeyboard);
}
export function EnableDialogModalOkBtn() {
    (document.getElementsByClassName("yp-modal-btn")[0]! as HTMLButtonElement).style.display = "block";
}
export function EnableDialogModalExit() {
    let modalDialog = document.getElementById("modalDialog")! as HTMLDivElement;
    modalDialog.removeAttribute("data-bs-backdrop");
    modalDialog.removeAttribute("data-bs-keyboard");
}