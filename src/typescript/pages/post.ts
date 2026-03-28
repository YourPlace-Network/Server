window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/post.scss";
import "../components/addPost";
import "../components/modalDialog";
import "../components/scrollTop";
import "../components/menu";
import { CreateCommentThread, type CommentSort } from "../components/commentThread";
import { CreatePostCard } from "../components/postCard";
import { HttpGetJson } from "../util/network";
import { ShowNotifications } from "../util/notifications";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}
    function main() {
        let DOM = {
            blockchain: (document.getElementById("postBlockchain") as HTMLInputElement).value,
            commentsContainer: document.getElementById("commentsContainer") as HTMLDivElement,
            commentSortBtn: document.getElementById("commentSortBtn") as HTMLButtonElement,
            commentSortItems: document.querySelectorAll("#commentSortControl .dropdown-item") as NodeListOf<HTMLAnchorElement>,
            postContainer: document.getElementById("postContainer") as HTMLDivElement,
            txHash: (document.getElementById("postTxHash") as HTMLInputElement).value,
        };
        const sortLabels: Record<CommentSort, string> = {
            dislikes: "Most Disliked",
            likes: "Most Liked",
            reactions: "Most Reacted",
            recent: "Recent",
        };
        let currentSort: CommentSort = getUrlSort();
        let currentPage: number = getUrlPage();
        let isPopState = false;
        let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl: HTMLElement) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});});
        ShowNotifications();
        loadPost();
        applySortUI(currentSort);
        history.replaceState({ cp: currentPage, sort: currentSort }, "");
        loadComments(currentPage);
        DOM.commentSortItems.forEach((item) => {
            item.addEventListener("click", (e) => {
                e.preventDefault();
                const sort = item.dataset.sort as CommentSort;
                if (sort === currentSort) return;
                currentSort = sort;
                currentPage = 0;
                applySortUI(sort);
                pushState(0, sort);
                loadComments(0);
            });
        });
        window.addEventListener("popstate", (e: PopStateEvent) => {
            if (e.state) {
                currentPage = e.state.cp || 0;
                currentSort = e.state.sort || "likes";
            } else {
                currentPage = getUrlPage();
                currentSort = getUrlSort();
            }
            applySortUI(currentSort);
            isPopState = true;
            loadComments(currentPage);
        });
        function applySortUI(sort: CommentSort) {
            DOM.commentSortItems.forEach((i) => i.classList.remove("active"));
            DOM.commentSortItems.forEach((i) => {
                if (i.dataset.sort === sort) i.classList.add("active");
            });
            DOM.commentSortBtn.innerHTML = `<i class="bi bi-sort-down"></i> ${sortLabels[sort]}`;
        }
        function getUrlPage(): number {
            const params = new URLSearchParams(window.location.search);
            const cp = parseInt(params.get("cp") || "0");
            return isNaN(cp) || cp < 0 ? 0 : cp;
        }
        function getUrlSort(): CommentSort {
            const params = new URLSearchParams(window.location.search);
            const sort = params.get("sort");
            if (sort === "dislikes" || sort === "likes" || sort === "reactions" || sort === "recent") return sort;
            return "likes";
        }
        async function loadPost() {
            const response = await HttpGetJson(`/post/data/${DOM.blockchain}/${DOM.txHash}`);
            if (response[0] !== 200 || !response[1] || !response[1].post) {
                DOM.postContainer.innerHTML = '<div class="postPageEmpty"><i class="bi bi-exclamation-circle"></i><p>Post not found</p></div>';
                return;
            }
            const postData = response[1].post;
            postData.likes = postData.reactions?.likes || 0;
            postData.dislikes = postData.reactions?.dislikes || 0;
            const postCard = await CreatePostCard(postData);
            const existingThread = postCard.querySelector(".commentThreadContainer");
            if (existingThread) {
                existingThread.remove();
            }
            DOM.postContainer.appendChild(postCard);
        }
        function loadComments(initialPage: number) {
            DOM.commentsContainer.innerHTML = "";
            const thread = CreateCommentThread({
                blockchain: DOM.blockchain,
                initialPage: initialPage,
                onPageChange: (page: number) => {
                    currentPage = page;
                    if (isPopState) {
                        isPopState = false;
                        return;
                    }
                    pushState(page, currentSort);
                },
                parentTxHash: DOM.txHash,
                sort: currentSort,
            });
            DOM.commentsContainer.appendChild(thread);
        }
        function pushState(page: number, sort: CommentSort) {
            const params = new URLSearchParams();
            if (page > 0) params.set("cp", page.toString());
            if (sort !== "likes") params.set("sort", sort);
            const qs = params.toString();
            const url = window.location.pathname + (qs ? "?" + qs : "");
            history.pushState({ cp: page, sort: sort }, "", url);
        }
    }
})();
