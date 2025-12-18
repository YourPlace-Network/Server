export let YP = {
    enroll: function(url: string): string {
        const payload = {
            e: url
        }
        return `yp/1/e:${JSON.stringify(payload)}`
    },
    metadataAvatar: function(url: string): string {
        const payload = {
            a: url
        }
        return `yp/1/ma:${JSON.stringify(payload)}`
    },
    metadataBanner: function(url: string): string {
        const payload = {
            b: url
        }
        return `yp/1/mb:${JSON.stringify(payload)}`
    },
    metadataDescription: function(description: string): string {
        const payload = {
            d: description
        }
        return `yp/1/md:${JSON.stringify(payload)}`
    },
    metadataLocation: function(location: string): string {
        const payload = {
            l: location
        }
        return `yp/1/ml:${JSON.stringify(payload)}`
    },
    metadataWebsite: function(website: string): string {
        const payload = {
            w: website
        }
        return `yp/1/mw:${JSON.stringify(payload)}`
    },
    metadataName: function(name: string): string {
        const payload = {
            n: name
        }
        return `yp/1/mn:${JSON.stringify(payload)}`
    },
    metadataVertical: function(vertical: string): string {
        const payload = {
            v: vertical
        }
        return `yp/1/mv:${JSON.stringify(payload)}`
    },
    post: function(post: string): string {
        const payload = {
            p: post
        }
        return `yp/1/p:${JSON.stringify(payload)}`
    },
    postAttach: function(post: string, attach: string[][]): string {
        const payload = {
            p: post,
            a: attach
        }
        return `yp/1/pa:${JSON.stringify(payload)}`
    },
    follow: function(toAddress: string, toBlockchain: string): string {
        const payload = {
            a: toAddress,
            b: toBlockchain,
        }
        return `yp/1/f:${JSON.stringify(payload)}`
    },
    unfollow: function(toAddress: string, toBlockchain: string): string {
        const payload = {
            a: toAddress,
            b: toBlockchain,
        }
        return `yp/1/fu:${JSON.stringify(payload)}`
    },
    comment: function(parentTxHash: string, text: string): string {
        const payload = {
            t: parentTxHash,
            p: text
        }
        return `yp/1/c:${JSON.stringify(payload)}`
    },
    commentAttach: function(parentTxHash: string, text: string, attach: string[][]): string {
        const payload = {
            t: parentTxHash,
            p: text,
            a: attach
        }
        return `yp/1/ca:${JSON.stringify(payload)}`
    },
    dislike: function(targetTxHash: string, targetType: string): string {
        const payload = {
            t: targetTxHash,
            y: targetType
        }
        return `yp/1/rdl:${JSON.stringify(payload)}`
    },
    emojiReact: function(targetTxHash: string, targetType: string, emoji: string): string {
        const payload = {
            t: targetTxHash,
            y: targetType,
            e: emoji
        }
        return `yp/1/re:${JSON.stringify(payload)}`
    },
    like: function(targetTxHash: string, targetType: string): string {
        const payload = {
            t: targetTxHash,
            y: targetType
        }
        return `yp/1/rl:${JSON.stringify(payload)}`
    },
}
