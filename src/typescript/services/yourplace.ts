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
    marketplaceListing: function(title: string, description: string, price: string, priceSmallUnit: string, currencySymbol: string, imageUrls: string[] = [], listingType: string = "fixed"): string {
        const payload = {
            t: title,
            d: description,
            p: price,
            psu: priceSmallUnit,
            c: currencySymbol,
            img: imageUrls,
            lt: listingType
        }
        return `yp/1/ml:${JSON.stringify(payload)}`
    },
    marketplaceOffer: function(listingTxHash: string, offerPrice: string, offerPriceSmallUnit: string, message?: string): string {
        const payload = {
            l: listingTxHash,
            op: offerPrice,
            opsu: offerPriceSmallUnit,
            m: message || ""
        }
        return `yp/1/mo:${JSON.stringify(payload)}`
    },
    marketplaceOfferAccept: function(offerTxHash: string): string {
        const payload = {
            o: offerTxHash
        }
        return `yp/1/moa:${JSON.stringify(payload)}`
    },
    marketplacePayment: function(offerAcceptTxHash: string, price: string, priceSmallUnit: string): string {
        const payload = {
            oa: offerAcceptTxHash,
            p: price,
            psu: priceSmallUnit
        }
        return `yp/1/mp:${JSON.stringify(payload)}`
    },
    marketplaceReceipt: function(paymentTxHash: string): string {
        const payload = {
            pt: paymentTxHash
        }
        return `yp/1/mr:${JSON.stringify(payload)}`
    },
    marketplaceListingCancel: function(listingTxHash: string, reason?: string): string {
        const payload = {
            l: listingTxHash,
            r: reason || ""
        }
        return `yp/1/mlc:${JSON.stringify(payload)}`
    },
    marketplaceOfferCancel: function(offerTxHash: string, reason?: string): string {
        const payload = {
            o: offerTxHash,
            r: reason || ""
        }
        return `yp/1/moc:${JSON.stringify(payload)}`
    },
    marketplaceAuctionListing: function(title: string, description: string, startPrice: string, startPriceSmallUnit: string, reservePrice: string, reservePriceSmallUnit: string, currencySymbol: string, duration: number, imageUrls: string[] = []): string {
        const payload = {
            t: title,
            d: description,
            sp: startPrice,
            spsu: startPriceSmallUnit,
            rp: reservePrice,
            rpsu: reservePriceSmallUnit,
            c: currencySymbol,
            dur: duration,
            img: imageUrls
        }
        return `yp/1/mal:${JSON.stringify(payload)}`
    },
}
