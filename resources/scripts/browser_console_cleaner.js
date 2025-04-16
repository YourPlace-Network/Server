// Console filter to hide "Too Many Requests" errors from mainnet.base.org
(function() {
    const originalConsoleError = console.error;

    console.error = function(...args) {
        // Check if this is a network error message we want to filter
        const errorString = args.join(" ");
        const isBaseOrgError = errorString.includes("mainnet.base.org") &&
            errorString.includes("429") &&
            errorString.includes("Too Many Requests");

        // Only show errors that don't match our filter criteria
        if (!isBaseOrgError) {
            originalConsoleError.apply(console, args);
        }
    };
})();