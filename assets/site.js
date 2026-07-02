document.addEventListener("DOMContentLoaded", function () {
    var observer = new IntersectionObserver(
        function (entries) {
            entries.forEach(function (entry) {
                if (entry.isIntersecting) {
                    entry.target.classList.add("visible");
                    observer.unobserve(entry.target);
                }
            });
        },
        { threshold: 0.01 },
    );
    document.querySelectorAll(".reveal").forEach(function (el) {
        observer.observe(el);
    });

    var header = document.querySelector(".site-header");
    if (header) {
        var lastY = window.scrollY;
        var hideThreshold = header.offsetHeight;
        var suppressHide = false;
        var suppressTimer = null;

        window.addEventListener(
            "scroll",
            function () {
                var y = window.scrollY;
                if (!suppressHide && y > lastY && y > hideThreshold) {
                    header.classList.add("hidden");
                } else {
                    header.classList.remove("hidden");
                }
                lastY = y;
            },
            { passive: true },
        );
        document.addEventListener("mousemove", function (event) {
            if (event.clientY <= 12) {
                header.classList.remove("hidden");
            }
        });

        // An in-page anchor jump (e.g. clicking "Status" in the nav) triggers
        // a smooth scroll that the listener above would otherwise read as
        // "scrolling down" and hide the header mid-jump -- but
        // scroll-padding-top still reserves space for it, so the target
        // lands short. Keep the header visible for the duration of the jump.
        document.querySelectorAll('a[href*="#"]').forEach(function (link) {
            link.addEventListener("click", function () {
                suppressHide = true;
                header.classList.remove("hidden");
                if (suppressTimer) clearTimeout(suppressTimer);
                suppressTimer = setTimeout(function () {
                    suppressHide = false;
                }, 1000);
            });
        });
    }

    // Mega-menu dropdowns (Project, Labs, Docs) open on CSS :hover/
    // :focus-within alone -- no JS state to fall out of sync. Escape still
    // needs JS: it can't un-hover, so blur whatever's focused inside one.
    document.addEventListener("keydown", function (event) {
        if (event.key !== "Escape") return;
        var active = document.activeElement;
        if (active && active.closest(".nav-dropdown")) active.blur();
    });
});
