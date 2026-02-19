window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/postCard.scss";
import DOMPurify from "dompurify";
import {HttpGetJson} from "../util/network";

export async function FetchPosts(blockchain: string, address: string, limit: number, offset: number): Promise<{posts: any[], totalCount: number} | null> {
    let resp = await HttpGetJson(`/posts/${blockchain}/${address}?limit=${limit}&offset=${offset}`);
    if (resp[0] === 200) {
        return {posts: resp[1].posts || [], totalCount: resp[1].totalCount || 0};
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