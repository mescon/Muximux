jQuery(document).ready(function($){
    var tabs = $('.cd-tabs');

    tabs.each(function(){
        var tab = $(this),
            tabItems = tab.find('ul.cd-tabs-navigation, .main-nav'),
            tabContentWrapper = tab.children('ul.cd-tabs-content'),
            tabNavigation = tab.find('nav');

        tabItems.on('click', 'a', function(event){
            event.preventDefault();
            var selectedItem = $(this);
            if( !selectedItem.hasClass('selected') ) {
                var selectedTab = selectedItem.data('content'),
                    selectedContent = tabContentWrapper.find('li[data-content="'+selectedTab+'"]'),
                    selectedContentHeight = selectedContent.innerHeight();

                selectedItem.dblclick(function() {
                    selectedContent.children('iframe').attr('src', selectedContent.children('iframe').attr('src'));
                })

                var sifsrc = selectedContent.children('iframe').attr('src');
                if (sifsrc === undefined || sifsrc === "") {
                    selectedContent.children('iframe').attr('src', selectedContent.children('iframe').data('src'));
                }

                tabItems.find('a.selected').removeClass('selected');
                selectedItem.addClass('selected');
                selectedContent.addClass('selected').siblings('li').removeClass('selected');
                //animate tabContentWrapper height when content changes
                tabContentWrapper.animate({
                    'height': selectedContentHeight
                }, 200);
            }
        });

        //hide the .cd-tabs::after element when tabbed navigation has scrolled to the end (mobile version)
        checkScrolling(tabNavigation);
        tabNavigation.on('scroll', function(){
            checkScrolling($(this));
        });
    });


    $('#reload').on('click', function(){
        var selectedFrame = $('.cd-tabs-content').find('.selected').children('iframe');
        selectedFrame.attr('src', selectedFrame.attr('src'));
    })


    $(window).on('resize', function(){
        tabs.each(function(){
            var tab = $(this);
            checkScrolling(tab.find('nav'));
            tab.find('.cd-tabs-content').css('height', 'auto');
        });

        if($(this).width() < 1024) { // Hide dropdown-menu if we're on a small screen.
                $('.dropdown').hide();
              } else {
                $('.dropdown').show();
        }
        resizeIframe(); // Resize iframes when window is resized.
    });


    function checkScrolling(tabs){
        var totalTabWidth = parseInt(tabs.children('.cd-tabs-navigation').width()),
            tabsViewport = parseInt(tabs.width());
        if(tabs.scrollLeft() >= totalTabWidth - tabsViewport) {
            tabs.parent('.cd-tabs').addClass('is-ended');
        } else {
            tabs.parent('.cd-tabs').removeClass('is-ended');
        }
    }

    // Measure viewport and subtract the height the navigation tabs, then resize the iframes.
    function resizeIframe(){
        var newSize = $(window).height() - $('nav').height();
        $('iframe').css({ 'height': newSize + 'px' });
    }

    $('.main-nav').hover(function (){
                dropDownFixPosition($('.main-nav'),$('.drop-nav'));
            });

    function dropDownFixPosition(button,dropdown){
          var dropDownTop = button.offset().top + button.outerHeight();
            dropdown.css('top', dropDownTop + "px");
            dropdown.css('left', $(window).width()-$('.drop-nav').width() - button.offset().left + "px");
    }

// Call resizeIframe when document is ready
resizeIframe();
});
