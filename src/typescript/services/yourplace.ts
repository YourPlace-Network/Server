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
    metadataBirthday: function(birthday: string): string {
        const payload = {
            bd: birthday
        }
        return `yp/1/mbd:${JSON.stringify(payload)}`
    },
    metadataName: function(name: string): string {
        const payload = {
            n: name
        }
        return `yp/1/mn:${JSON.stringify(payload)}`
    },
    post: function(post: string): string {
        const payload = {
            p: post
        }
        return `yp/1/p:${JSON.stringify(payload)}`
    },
    postAttach: function(post: string, attach: string[]): string {
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
}
