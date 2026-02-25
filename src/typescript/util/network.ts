import {LogError} from "./log";

export async function HttpGetJson(url: string): Promise<[number, any]>{
    const options = {
        method: "GET",
        cache: "no-store",
        headers: {
            "Accept": "application/json",
            "Content-Type": "application/json; charset=utf-8"
        }
    }
    let response: Response;
    try {
        response = await fetchWithTimeout(url, options, 30000);
    } catch (error: any) {
        return [400, {"status": "Request timed out"}];
    }
    if (!response.ok) {
        return [response.status, null];
    }
    try {
        return [response.status, await response.json()];
    } catch (e) {
        return [response.status, null];
    }
}
export async function HttpPostJson(url: string, payload: any, csrfToken: string): Promise<[number, any]> {
    if (csrfToken == null || csrfToken == "") {
        LogError("HttpPostJson: Invalid/missing CSRF Token");
        return [400, {"status": "Invalid/missing CSRF Token"}];
    }
    const options = {
        method: "POST",
        cache: "no-store",
        headers: {
            "Accept": "application/json",
            "Content-Type": "application/json; charset=utf-8",
            "X-CSRF-Token": csrfToken,
        },
        body: JSON.stringify(payload),
        credentials: "include", // Important for CSRF cookies
    };
    let response: Response;
    try {
        response = await fetchWithTimeout(url, options, 30000);
    } catch (error: any) {
        return [400, {"status": "Request timed out"}];
    }
    if (!response.ok) {
        try {
            const errorText = await response.text();
            return [response.status, errorText ? JSON.parse(errorText) : null];
        } catch (e) {
            return [response.status, null];
        }
    }
    try {
        const responseData = await response.json();
        return [response.status, responseData];
    } catch (e) {
        return [response.status, null];
    }
}
export async function HttpPostFile(url: string, file: File | FileList, csrfToken: string): Promise<[number, any]> {
    if (csrfToken == null || csrfToken == "") {
        return [400, {"status": "Invalid CSRF Token"}];
    }
    let formData = new FormData();
    if (file instanceof FileList) {
        for (let i = 0; i < file.length; i++) {
            formData.append("file", file[i]);
        }
    } else {
        formData.append("file", file);
    }
    const options = {
        method: "POST",
        cache: "no-store",
        body: formData,
        headers: {
            "Accept": "application/json",
            "X-CSRF-Token": csrfToken,
        },
        credentials: "include", // Important for CSRF cookies
    }
    let response: Response;
    try {
        response = await fetchWithTimeout(url, options, 120000);
    } catch (error: any) {
        return [400, {"status": "Request timed out"}];
    }
    if (!response.ok) {
        try {
            const errorText = await response.text();
            return [response.status, errorText ? JSON.parse(errorText) : null];
        } catch (e) {
            return [response.status, null];
        }
    }
    try {
        return [response.status, await response.json()];
    } catch (e) {
        return [response.status, null];
    }
}
export async function CnameResolve(domain: string): Promise<string | null> {
    const dohURL = `https://dns.google.com/resolve?name=${domain}&type=CNAME`;
    const response = await fetch(dohURL);
    if (!response.ok) {
        console.log(`DNS CNAME resolve failed for ${domain}`);
        return null;
    }
    const data = await response.json();
    const answer = data.Answer && data.Answer[0];
    if (answer && answer.type === 5) {
        return answer.data;
    }
    return null;
}

async function fetchWithTimeout(url: string, options = {}, timeout = 5000): Promise<Response> {
    const fullUrl = url.startsWith("http") ? url : `${window.location.origin}${url}`;
    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), timeout);
    try {
        const response = await fetch(fullUrl, {
            ...options,
            signal: controller.signal
        });
        clearTimeout(id);
        return response;
    } catch (error: any) {
        clearTimeout(id);
        if (error.name === "AbortError") {
            throw error;
        }
        throw error;
    }
}