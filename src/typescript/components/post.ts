window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/postCard.scss";
import DOMPurify from "dompurify";
import {HttpGetJson} from "../util/network";

export async function FetchPosts(blockchain: string, address: string): Promise<[] | null> { // retrieves a user's posts from the backend
    let resp = await HttpGetJson("/posts/" + blockchain + "/" + address);
    if (resp[0] === 200) {
        let posts = resp[1].posts;
        return posts;
    }
    return null;
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});});
    }
})();