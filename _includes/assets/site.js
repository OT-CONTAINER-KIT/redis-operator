function feedback(type, message) {
    console.log(`feedback: https://github.com/rundocs/jekyll-rtd-theme/issues?q=${type}+${message}`);
}

function highlight() {
    let text = new URL(location.href).searchParams.get("highlight");
    let regexp = new RegExp(text, "im");

    if (text) {
        $(".section").find("*").each(function() {
            if (this.outerHTML.match(regexp)) {
                $(this).addClass("highlighted-box");
            }
        });
        $(".section").find(".highlighted-box").each(function() {
            if (($(this).find(".highlighted-box").length > 0)) {
                $(this).removeClass("highlighted-box");
            }
        });
    }
}

function search(data) {
    let text = new URL(location.href).searchParams.get("q");
    let results = [];
    let regexp = new RegExp(text, "im");

    function slice(content, min, max) {
        return content.slice(min, max).replace(regexp, (match) => `<span class="highlighted">${match}</span>`);
    }
    for (page of data) {
        let [title, content] = [null, null];
        try {
            if (page.title) {
                title = page.title.match(regexp);
            } else {
                if (page.url == "/") {
                    page.title = "{{ site.title }}";
                } else {
                    page.title = page.url;
                }
            }
        } catch (e) {
            feedback("search", e.message);
        }
        try {
            if (page.content) {
                page.content = $("<div/>").html(page.content).text();
                content = page.content.match(regexp);
            }
        } catch (e) {
            feedback("search", e.message);
        }
        if (title || content) {
            let result = [`<a href="{{ site.baseurl }}${page.url}?highlight=${text}">${page.title}</a>`];
            if (content) {
                let [min, max] = [content.index - 100, content.index + 100];
                let [prefix, suffix] = ["...", "..."];

                if (min < 0) {
                    prefix = "";
                    min = 0;
                }
                if (max > page.content.length) {
                    suffix = "";
                    max = page.content.length;
                }
                result.push(`<p class="context">${prefix}${slice(page.content ,min, max)}${suffix}</p>`);
            }
            results.push(`<li>${result.join("")}</li>`);
        }
    }
    if (results.length > 0 && text.length > 0) {
        $("#search-results ul.search").html(results.join(""));
        $("#search-results p.search-summary").html("{{ __search_results_found }}".replace("#", results.length));
    } else {
        $("#search-results ul.search").empty();
        $("#search-results p.search-summary").html("{{ __search_results_not_found }}");
    }
    $("#rtd-search-form [name='q']").val(text);
    $("#search-results h2").html("{{ __search_results }}");
}

function reset() {
    const link = $(".wy-menu-vertical").find(`[href="${location.pathname}"]`);
    if (link.length > 0) {
        $(".wy-menu-vertical .current").removeClass("current");
        link.addClass("current");
        link.closest("li.toctree-l1").parent().addClass("current");
        link.closest("li.toctree-l1").addClass("current");
        link.closest("li.toctree-l2").addClass("current");
        link.closest("li.toctree-l3").addClass("current");
        link.closest("li.toctree-l4").addClass("current");
        link.closest("li.toctree-l5").addClass("current");
    }
}

function admonition() {
    const items = {
        note: "{{ __note }}",
        tip: "{{ __tip }}",
        warning: "{{ __warning }}",
        danger: "{{ __danger }}"
    };
    for (let item in items) {
        let content = $(`.language-${item}`).html();
        $(`.language-${item}`).replaceWith(`<div class="admonition ${item}"><p class="admonition-title">${items[item]}</p><p>${content}</p></div>`);
    }
}

$(document).ready(function() {
    if (location.pathname == "{{ site.baseurl }}/search.html") {
        $.ajax({
                dataType: "json",
                url: "{{ site.baseurl }}/pages.json"
            })
            .done(search)
            .fail((xhr, message) => feedback("search", message));
    }
    admonition();
    anchors.add();
    highlight();
    SphinxRtdTheme.Navigation.reset = reset;
    SphinxRtdTheme.Navigation.enable(true);
});