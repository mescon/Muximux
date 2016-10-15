jQuery(document).ready(function ($) {
    // Custom function to do case-insensitive selector matching
    $.extend($.expr[":"], {
        "containsInsensitive": function(elem, i, match, array) {
        return (elem.textContent || elem.innerText || "").toLowerCase().indexOf((match[3] || "").toLowerCase()) >= 0;
        }
    });


    var tabs = $('.cd-tabs');

    // Set default title to the selected item on load
    var activeTitle = $('li .selected').attr("data-title");
    setTitle(activeTitle);

    tabs.each(function () {
        var tab = $(this),
            tabItems = tab.find('ul.cd-tabs-navigation, .main-nav'),
            tabContentWrapper = tab.children('ul.cd-tabs-content'),
            tabNavigation = tab.find('nav');

        tabItems.on('click', 'a:not(#reload, #hamburger)', function (event) {
            resizeIframe(); // Call resizeIframe when document is ready
            event.preventDefault();
            var selectedItem = $(this);
            if (!selectedItem.hasClass('selected')) {
                var selectedTab = selectedItem.data('content'),
                    selectedContent = tabContentWrapper.find('li[data-content="' + selectedTab + '"]'),
                    selectedContentHeight = selectedContent.innerHeight();

                selectedItem.dblclick(function () {
                    selectedContent.children('iframe').attr('src', selectedContent.children('iframe').attr('src'));
                });

                var sifsrc = selectedContent.children('iframe').attr('src');
                if (sifsrc === undefined || sifsrc === "") {
                    selectedContent.children('iframe').attr('src', selectedContent.children('iframe').data('src'));
                }

                tabItems.find('a.selected').removeClass('selected');
                selectedItem.addClass('selected');

                // Change window title after class "selected" has been added to item
                var activeTitle = selectedItem.attr("data-title");
                setTitle(activeTitle);

                selectedContent.addClass('selected').siblings('li').removeClass('selected');
                // animate tabContentWrapper height when content changes
                tabContentWrapper.animate({
                    'height': selectedContentHeight
                }, 200);
            }
        });

        // hide the .cd-tabs::after element when tabbed navigation has scrolled to the end (mobile version)
        checkScrolling(tabNavigation);
        tabNavigation.on('scroll', function () {
            checkScrolling($(this));
        });
    });

    // Keep hamburger-menu white when hovering items UNDER the hamburger menu.
    $('.drop-nav').on('mouseover', function () {
        $('.main-nav a span:first').addClass('dd-active');
    });

    $('.drop-nav').on('mouseout', function () {
        $('.main-nav a span:first').removeClass('dd-active');
    });


    $('#reload').on('click', function () {
        var selectedFrame = $('.cd-tabs-content').find('.selected').children('iframe');
        selectedFrame.attr('src', selectedFrame.attr('src'));
    });

    // When settings modal is open, set title to "Settings"
    $('#settingsModal').on('show.bs.modal', function () {
        setTitle("Settings");
    });

    // When settings modal closes, set title to the previous title used
    $('#settingsModal').on('hidden.bs.modal', function () {
        var activeTitle = $('.cd-tabs-content').find('.selected').children('iframe').attr("data-title");
        setTitle(activeTitle);
    });



    $(window).on('resize', function () {
        tabs.each(function () {
            var tab = $(this);
            checkScrolling(tab.find('nav'));
            tab.find('.cd-tabs-content').css('height', 'auto');
        });

        resizeIframe(); // Resize iframes when window is resized.
        scaleFrames(); // Scale frames when window is resized.
    });

    $('.main-nav').hover(function () {
        dropDownFixPosition($('.main-nav'), $('.drop-nav'));
    });
    jQuery.fn.reverse = [].reverse;

     // Move items to the dropdown on mobile devices
    muximuxMobileResize();

    settingsEventHandlers();
    scaleFrames();
    resizeIframe(); // Call resizeIframe when document is ready
    initIconPicker('.iconpicker');
    getSecret();
    getBranch();
    getGitHubData();
    var commands = ["hash","cwd","phpini","gitdirectory", "title", "greeting"];
    getSystemData(commands);


    // Load the menu item that is set in URL, for example http://site.com/#plexpy
    if($(location).attr('hash')) {
        var bookmarkHash = $(location).attr('hash').substr(1).replace("%20", " ").replace("_", " ");
        var menuItem = $(document).find('a:containsInsensitive("'+bookmarkHash+'")');
        menuItem.trigger("click");
    }

});


// When user closes the page, create new unique ID in secret.txt so that the token is no longer valid if used after page load.
$(window).unload(function() {
      $.ajax({
        async: true,
        dataType: 'text',
        url: "muximux.php?secret=" + dataStore().secret + "&set=secret",
        type: 'GET'
    });
});

$( window ).resize(muximuxMobileResize);

function muximuxMobileResize() {
    if($( window ).width() < 800) {
        // isMobile = true;
        $('.cd-tabs-navigation nav').children().appendTo(".drop-nav");
        var menuHeight = $( window ).height() * .80;
        $('.drop-nav').css('max-height', menuHeight+'px');
    } else {
        // isMobile = false;
        $(".drop-nav").children('.cd-tab').appendTo('.cd-tabs-navigation nav');
        $('.drop-nav').css('max-height', '');
    }
}