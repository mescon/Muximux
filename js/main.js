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

    function settings_Init() {
        //Defaults
        $('.appName').attr('disabled', 'disabled');
        $('#sortable').sortable({
            change: function () {
                $('form').prepend('<input type="hidden" name="reorder" value="true">');
            }
        });
        $('select').each(function () {
            $(this).parents('.applicationContainer').children('.example_icon').addClass($(this).val());
        });
        initButtonHandlers('.saveApp', '.removeButton', 0);


        //Change handlers
        $('input[type=radio]').change(function () {
            // When any radio button on the page is selected,
            // then deselect all other radio buttons.
            $('input[type=radio]:checked').not(this).prop('checked', false);
        });
        $('.iconDD').change(function () {
            $(this).parents('.applicationContainer').children('span').attr('class', '').attr('class', 'example_icon ' + $(this).val());
        });


        $('#refresh-page').click(function () {
            location.pathname = location.pathname;
        });

        //Add new application button
        $('#addApplication').click(function () {
            //Generating a random number here. So that if the user adds more than one new appliation at a time the ids/classes and names dont match.
            var rand = Math.floor((Math.random() * 999999) + 1);
            $('#sortable').prepend(
                '<div class="applicationContainer newApp" id="' + rand + 'newApplication">' +
                '<input type="button" class="saveApp" id="' + rand + 'saveApp-newApplication" value="Save Application Name">' +
                '<div>name:<input class="appName ' + rand + 'newApplication-value" name="' + rand + 'newApplication-name" type="text" value=""></div>' +
                '<div>url:<input class="' + rand + 'newApplication-value" name="' + rand + 'newApplication-url" type="text" value=""></div>' +
                '<div>icon:<input class="' + rand + 'newApplication-value" name="' + rand + 'newApplication-icon" type="text" value="fa fa-globe"></div>' +
                '<div>enabled:<input class="checkbox ' + rand + 'newApplication-value" name="' + rand + 'newApplication-enabled" type="checkbox"></div>' +
                '<div>default:<input class="checkbox ' + rand + 'newApplication-value" name="' + rand + 'newApplication-default" type="checkbox"></div>' +
                '<div>landingpage:<input class="checkbox ' + rand + 'newApplication-value" name="rutorrent-landingpage" type="checkbox"></div>' +
                '<div>dd:<input class="checkbox ' + rand + 'newApplication-value" name="rutorrent-dd" type="checkbox"></div>' +

                    //'<input type="button" class="saveButton" value="Save" id="save-'+rand+'newApplication">' +
                '<input type="button" class="removeButton" value="Remove" id="remove-' + rand + 'newApplication"></div></div>');
            initButtonHandlers('#' + rand + 'saveApp-newApplication', '#remove-' + rand + 'newApplication', rand);
        });
    }

    function initButtonHandlers(saveApplicationButton, removeButton, rand) {
        //Update Application Name Button Handler
        $(saveApplicationButton).click(function () {
            $(this).siblings().children('.appName').removeAttr('disabled');
            var applicationInput = $(this).siblings().children('.appName');
            if ($(this).attr('value') == "Update Application Name") {
                applicationInput.removeAttr('disabled');
                $(this).attr('value', 'Save Application Name');
            }
            else {
                var section = applicationInput.attr('was');
                if (section == undefined)
                    section = rand + "newApplication";
                else
                    $('#removed').append('<input type="hidden" name="removed-' + section + '" value="Removed">');

                var newSection = $(this).siblings().children('.appName').val().split(' ').join('_');
                applicationInput.attr('was', newSection);
                applicationInput.val(newSection);
                applicationInput.attr('value', newSection);
                $('.' + section + '-value').each(function () {
                    var split = $(this).attr('name').split('-');
                    $(this).removeAttr('name')
                        .prop('name', newSection + "-" + split[1])
                        .addClass(newSection + '-value')
                        .removeClass(section + '-value');
                });
                $(this).attr('value', 'Update Application Name');
                applicationInput.attr('disabled', 'disabled');
                $(this).parents('div.applicationContainer').attr('id', newSection);
            }
        });
        //Remove Button Handler
        $(removeButton).click(function () {
            if (confirm('Are you sure?')) {
                runRemove($(this).parent('div.applicationContainer'));
                var removedApp = $(this).parent('div.applicationContainer').attr('id');
                $('#removed').append('<input type="hidden" name="removed-' + removedApp + '" value="Removed">');
            }
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
    }

    function settings_addFalseCheckboxes() {
        var options = {
            url: 'settings.php',
            type: 'post',
            success: showResponse
        };
        $('#settingsSubmit').click(function (event) {
            event.preventDefault();
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
        if (responseText == 1) {
            $('#saved').show(1000);
            setTimeout(function () {
                $('#saved').hide(1000);
            }, 2000);
        }
    }

    settings_addFalseCheckboxes();
    settings_Init();
    resizeIframe(); // Call resizeIframe when document is ready
//hideDropdownMenu(); // Check if we should hide the dropdown menu
});
