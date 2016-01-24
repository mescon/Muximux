jQuery(document).ready(function ($) {
    var tabs = $('.cd-tabs');

    tabs.each(function () {
        var tab = $(this),
            tabItems = tab.find('ul.cd-tabs-navigation, .main-nav'),
            tabContentWrapper = tab.children('ul.cd-tabs-content'),
            tabNavigation = tab.find('nav');

        tabItems.on('click', 'a', function (event) {
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
                selectedContent.addClass('selected').siblings('li').removeClass('selected');
                //animate tabContentWrapper height when content changes
                tabContentWrapper.animate({
                    'height': selectedContentHeight
                }, 200);
            }
        });

        //hide the .cd-tabs::after element when tabbed navigation has scrolled to the end (mobile version)
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


    $(window).on('resize', function () {
        tabs.each(function () {
            var tab = $(this);
            checkScrolling(tab.find('nav'));
            tab.find('.cd-tabs-content').css('height', 'auto');
        });

        //hideDropdownMenu(); // Check if we should hide the dropdown menu.
        resizeIframe(); // Resize iframes when window is resized.
    });

    $('.main-nav').hover(function () {
        dropDownFixPosition($('.main-nav'), $('.drop-nav'));
    });

    /*function hideDropdownMenu() {
     if($(window).width() < 1024) {
     $('.main-nav').hide();
     } else {
     $('.main-nav').show();
     }
     }*/
    settingsInit();
    settingsPost();
    resizeIframe(); // Call resizeIframe when document is ready
    //hideDropdownMenu(); // Check if we should hide the dropdown menu
});


function checkScrolling(tabs) {
    var totalTabWidth = parseInt(tabs.children('.cd-tabs-navigation').width()),
        tabsViewport = parseInt(tabs.width());
    if (tabs.scrollLeft() >= totalTabWidth - tabsViewport) {
        tabs.parent('.cd-tabs').addClass('is-ended');
    } else {
        tabs.parent('.cd-tabs').removeClass('is-ended');
    }
}

// Measure viewport and subtract the height the navigation tabs, then resize the iframes.
function resizeIframe() {
    var newSize = $(window).height() - $('nav').height();
    $('iframe').css({'height': newSize + 'px'});
}

function dropDownFixPosition(button, dropdown) {
    var dropDownTop = button.offset().top + button.outerHeight();
    dropdown.css('top', dropDownTop + "px");
    dropdown.css('left', $(window).width() - $('.drop-nav').width() - button.offset().left + "px");
}

function settingsInit() {
    //Defaults
    $('#sortable').sortable();
    $('select').each(function () {
        $(this).siblings('.example_icon').addClass($(this).val());
    });

    $('input[type=radio]').change(function () {
        $('input[type=radio]:checked').not(this).prop('checked', false);
    });
    $('.iconDD').change(function () {
        $(this).siblings('span').attr('class', '').attr('class', 'example_icon ' + $(this).val());
    });


    $('#refresh-page').click(function () {
        location.pathname = location.pathname;
    });

    $('#showInstructions').click(function () {
        $('#instructions').slideToggle(1000);
        if ($(this).val() == "Show Instructions")
            $(this).val('Hide Instructions');
        else
            $(this).val('Show Instructions');

    });

    //Remove Button Handler
    $('form').on('click', '.removeButton', function () {
        if (confirm('Are you sure?')) {
            var selectedEffect = "drop";
            var options = {};
            $($(this).parents('.applicationContainer')).effect(selectedEffect, options, 500, removeCallback($(this).parents('.applicationContainer')));
        }
    });
    function removeCallback(selectedElement) {
        setTimeout(function () {
            $(selectedElement).remove();
        }, 1000);
    };

    //Add new application button
    $('#addApplication').click(function () {
        //Generating a random number here. So that if the user adds more than one new application at a time the ids/classes and names don't match.
        var rand = Math.floor((Math.random() * 999999) + 1);
        $('#sortable').append(
            '<div class="applicationContainer newApp" id="' + rand + 'newApplication"><span class="bars fa fa-bars"></span>' +
            '<div>Name:<input class="appName ' + rand + 'newApplication-value" name="' + rand + 'newApplication-name" type="text" value=""></div>' +
            '<div>URL:<input class="' + rand + 'newApplication-value" name="' + rand + 'newApplication-url" type="text" value=""></div>' +
            '<div>Icon:<select class="newApp-Icon ' + rand + 'newApplication-value" name="' + rand + 'newApplication-icon" value="fa fa-globe"></select></div>' +
            '<div>Enable:<input class="checkbox ' + rand + 'newApplication-value" name="' + rand + 'newApplication-enabled" type="checkbox" checked></div>' +
            '<div>Default:<input class="radio ' + rand + 'newApplication-value" name="' + rand + 'newApplication-default" type="radio"></div>' +
            '<div>Landing page:<input class="checkbox ' + rand + 'newApplication-value" name="rutorrent-landingpage" type="checkbox"></div>' +
            '<div>Put in dropdown:<input class="checkbox ' + rand + 'newApplication-value" name="rutorrent-dd" type="checkbox"></div>' +
            '<button type="button" class="removeButton btn btn-danger btn-xs" value="Remove" id="remove-' + rand + 'newApplication">Remove<meta class="newAppRand" value="' + rand + '"></button><meta class="newAppRand" value="' + rand + '"></div></div>');
        $('.iconDD').first().children().clone().appendTo('.newApp-Icon');
    });

    //App Name Change/Addition
    $('form').on('focusout', '.appName', function () {
        $(this).parents('.applicationContainer').removeClass('newApp');
        var section = $(this).attr('was');
        if (section == undefined) {
            section = $(this).parents('.applicationContainer').children('.newAppRand').attr('value') + "newApplication";
            $(this).parents('applicationContainer').children('.newAppRand').remove();
        }

        var newSection = $(this).val().split(' ').join('_');
        $(this).attr('was', newSection);
        $(this).val(newSection);
        $(this).attr('value', newSection);
        $('.' + section + '-value').each(function () {
            var split = $(this).attr('name').split('-');
            $(this).removeAttr('name')
                .prop('name', newSection + "-" + split[1])
                .addClass(newSection + '-value')
                .removeClass(section + '-value');
        });
        $(this).parents('div.applicationContainer').attr('id', newSection);
    });
}

function settingsPost() {
    var options = {
        url: 'settings.php',
        type: 'post',
        success: showResponse
    };
    $('#settingsSubmit').click(function (event) {
        event.preventDefault();
        $('.newApp').remove(); //Remove new app that isn't filled out.
        $('.checkbox,.radio').each(function () {
            if (!$(this).prop('checked')) {
                var name = $(this).attr('name');
                $('<input type="hidden" name="' + name + '" value="false">').appendTo($(this));
            }
        });
        $('.appName').removeAttr('disabled');
        $("form").ajaxSubmit(options);
    });
}

// post-submit callback
function showResponse(responseText, statusText) {
    if (responseText == 1)
        location.pathname = location.pathname;
    else
        alert("Error!!!-" + responseText);
}