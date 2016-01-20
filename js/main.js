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
        tabNavigation.on('scroll', function () {
            checkScrolling($(this));
        });
    });

    // Keep hamburger-menu white when hovering items UNDER the hamburger menu.
    $('.drop-nav').on('mouseover', function () {
        $('.main-nav a span:first').addClass('dd-active');
    })

    $('.drop-nav').on('mouseout', function () {
        $('.main-nav a span:first').removeClass('dd-active');
    })


    $('#reload').on('click', function () {
        var selectedFrame = $('.cd-tabs-content').find('.selected').children('iframe');
        selectedFrame.attr('src', selectedFrame.attr('src'));
    })


    $(window).on('resize', function () {
        tabs.each(function () {
            var tab = $(this);
            checkScrolling(tab.find('nav'));
            tab.find('.cd-tabs-content').css('height', 'auto');
        });

        //hideDropdownMenu(); // Check if we should hide the dropdown menu.
        resizeIframe(); // Resize iframes when window is resized.
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

    /*function hideDropdownMenu() {
     if($(window).width() < 1024) {
     $('.main-nav').hide();
     } else {
     $('.main-nav').show();
     }
     }*/

    $('.main-nav').hover(function () {
        dropDownFixPosition($('.main-nav'), $('.drop-nav'));
    });

    function dropDownFixPosition(button, dropdown) {
        var dropDownTop = button.offset().top + button.outerHeight();
        dropdown.css('top', dropDownTop + "px");
        dropdown.css('left', $(window).width() - $('.drop-nav').width() - button.offset().left + "px");
    }

    function settings_Buttons() {
        //Defaults
        $('.applicationName').attr('disabled', 'disabled');
        $('#sortable').sortable();
        initButtonHandlers();


        //Add new application button
        $('#addApplication').button().click(function () {
            $('#addApplication').attr('disabled','disabled');
            $('#sortable').prepend(
                '<li class="applicationContainer" id="newApplication"><div>Application: ' +
                '<input class="applicationName" was="newApplication" type="text" value="">' +
                '<input type="button" class="saveApp" value="Save"></div>' +
                '<div>enabled:<input class="checkbox newApplication-value" name="newApplication-enabled" type="checkbox"></div>' +
                '<div>default:<input class="checkbox newApplication-value" name="newApplication-default" type="checkbox"></div>' +
                '<div>name:<input class="newApplication-value" name="newApplication-name" type="text" value=""></div>' +
                '<div>url:<input class="newApplication-value" name="newApplication-url" type="text" value=""></div>' +
                '<div>landingpage:<input class="checkbox newApplication-value" name="rutorrent-landingpage" type="checkbox" checked=""></div>' +
                '<div>icon:<input class="newApplication-value" name="newApplication-icon" type="text" value="fa fa-globe"></div>' +
                '<input type="button" class="saveButton" value="Save" id="save-newApplication">' +
                '<input type="button" class="removeButton" value="Remove" id="remove-newApplication"></li>');
            initButtonHandlers();
        });
    }

    function initButtonHandlers(){
        //Update Application Name Button Handler
        $('.saveApp').button().click(function () {
            $('#addApplication').removeAttr('disabled');
            var applicationInput = $(this).prev();
            if ($(this).attr('value') == "Update Application Name") {
                applicationInput.removeAttr('disabled');
                $(this).attr('value', 'Save');
            }
            else {
                var section = $(this).prev().attr('was');
                var newSection = $(this).prev().val();
                applicationInput.attr('was', newSection);
                $('.' + section + '-value').each(function () {
                    var split = $(this).attr('name').split('-');
                    $(this).removeAttr('name')
                        .prop('name', newSection + "-" + split[1])
                        .addClass(newSection + '-value')
                        .removeClass(section + '-value');
                });
                $(this).attr('value', 'Update Application Name');
                applicationInput.attr('disabled', 'disabled');
                $(this).parents('li').attr('id',newSection)
            }
        });
        //Remove Button Handler
        $('.removeButton').button().click(function () {
            runRemove($(this).parent('li'))
        });
        function runRemove(selectedElement) {
            var selectedEffect = "drop";
            var options = {};
            $(selectedElement).effect(selectedEffect, options, 500, removeCallback(selectedElement));
        };
        function removeCallback(selectedElement) {
            setTimeout(function () {
                $(selectedElement).remove();
            }, 1000);
        };
        //Save button handler - Add ajax
        $('.saveButton').button();
    }

    function settings_addFalseCheckboxes() {
        $('#settingsSubmit').click(function (event) {
            event.preventDefault();
            $('.checkbox').each(function () {
                if (!$(this).prop('checked')) {
                    var name = $(this).attr('name');
                    $('form').append('<input type="hidden" name="' + name + '" value="false">');
                }
            });
            $('form').submit();
        });

    }

    settings_addFalseCheckboxes();
    settings_Buttons();
    resizeIframe(); // Call resizeIframe when document is ready
//hideDropdownMenu(); // Check if we should hide the dropdown menu
});
