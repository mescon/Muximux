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
    initIconPicker('.iconpicker');
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

    $('input[type=radio]').change(function () {
        $('input[type=radio]:checked').not(this).prop('checked', false);
    });

    $('#refresh-page').click(function () {
        location.pathname = location.pathname;
    });

    $('#showInstructions').click(function () {
        $('#instructionsContainer').slideToggle(1000);
        if ($(this).html() == "<span class=\"fa fa-book\"></span> Show Instructions")
            $(this).html('<span class=\"fa fa-book\"></span> Hide Instructions');
        else
            $(this).html('<span class=\"fa fa-book\"></span> Show Instructions');

    });

    $('#showChangelog').click(function () {
        $('#changelogContainer').slideToggle(1000);
        viewChangelog();
        if ($(this).html() == "<span class=\"fa fa-github\"></span> Show Updates")
            $(this).html('<span class=\"fa fa-github\"></span> Hide Updates');
        else
            $(this).html('<span class=\"fa fa-github\"></span> Show Updates');
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

    //Fix for iconpicker. For some reason the arrow doesn't get disabled when it hits the minimum/maximum page number. This disables the button, so that it doesnt go into the negatives or pages above its max.
    $('body').on('click','.btn-arrow',function(event){
        event.preventDefault();
        if($(this).hasClass('disabled'))
            $(this).attr('disabled','disabled');
        else
            $('.btn-arrow').removeAttr('disabled');

    });

    //Add new application button
    $('#addApplication').click(function () {
        //Generating a random number here. So that if the user adds more than one new application at a time the ids/classes and names don't match.
        var rand = Math.floor((Math.random() * 999999) + 1);
        $('#sortable').append(
            '<div class="applicationContainer newApp" id="' + rand + 'newApplication"><span class="bars fa fa-bars"></span>' +
            '<div><label>Name:</label><input class="appName ' + rand + 'newApplication-value" name="' + rand + 'newApplication-name" type="text" value=""></div>' +
            '<div><label>URL:</label><input class="' + rand + 'newApplication-value" name="' + rand + 'newApplication-url" type="text" value=""></div>' +
            '<div><label>Icon:</label><button class=\"'+rand+'newApplication-value iconpicker btn btn-default\" name="'+rand+'newApplication-icon"  data-iconset=\"fontawesome\" data-icon=\"\"></button></div>' +
            '<div><label for="'+rand+'newApplication-enabled">Enable:</label><input class="checkbox ' + rand + 'newApplication-value" id="' + rand + 'newApplication-enabled" name="' + rand + 'newApplication-enabled" type="checkbox" checked></div>' +
            '<div><label for="'+rand+'newApplication-default">Default:</label><input class="radio ' + rand + 'newApplication-value" id="' + rand + 'newApplication-default" name="' + rand + 'newApplication-default" type="radio"></div>' +
            '<div><label for="'+rand+'newApplication-landingpage">Enable landing page:</label><input class="checkbox ' + rand + 'newApplication-value" id="' + rand + 'newApplication-landingpage" name="newApplication-landingpage" type="checkbox"></div>' +
            '<div><label for="'+rand+'newApplication-dd">Put in dropdown:</label><input class="checkbox ' + rand + 'newApplication-value" id="' + rand + 'newApplication-dd" name="newApplication-dd" type="checkbox"></div>' +
            '<button type="button" class="removeButton btn btn-danger btn-xs" value="Remove" id="remove-' + rand + 'newApplication">Remove<meta class="newAppRand" value="' + rand + '"></button><meta class="newAppRand" value="' + rand + '"></div></div>');
        initIconPicker('.'+rand+'newApplication-value[name='+rand+'newApplication-icon]');
    });

    //App Name Change/Addition
    $('form').on('focusout', '.appName', function () {
        if($(this).val() != "") {
            $(this).parents('.applicationContainer').removeClass('newApp');
            var section = $(this).attr('was');
            if (section == undefined) {
                section = $(this).parents('.applicationContainer').children('.newAppRand').attr('value') + "newApplication";
                $(this).parents('applicationContainer').children('.newAppRand').remove();
            }

            var newSection = $(this).val().split(' ').join('_');
            $(this).attr('was', newSection);
            $('.' + section + '-value').each(function () {
                var split = $(this).attr('name').split('-');
                if (split[1] == 'icon')
                    $(this).children('input').prop('name', newSection + "-" + split[1]);
                $(this).removeAttr('name')
                    .prop('name', newSection + "-" + split[1])
                    .addClass(newSection + '-value')
                    .removeClass(section + '-value');
            });
            $(this).parents('div.applicationContainer').attr('id', newSection);
        }
    });
}

function initIconPicker(selectedItem){
    $(selectedItem).iconpicker({
        align: 'center', // Only in div tag
        arrowClass: 'btn-danger',
        arrowPrevIconClass: 'glyphicon glyphicon-chevron-left',
        arrowNextIconClass: 'glyphicon glyphicon-chevron-right',
        cols: 10,
        footer: true,
        header: true,
        iconset: 'fontawesome',
        labelHeader: '{0} of {1} pages',
        labelFooter: '{0} - {1} of {2} icons',
        placement: 'bottom', // Only in button tag
        rows: 5,
        search: true,
        searchText: 'Search',
        selectedClass: 'btn-success',
        unselectedClass: ''
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


function datediff(latestDate) {
  var rightNow = new Date();
  var currentDate = rightNow.toISOString().substring(0,10).split('-').join('');
  var test = latestDate.split('-').join('');
  return currentDate-test;
}

function getLocalVersion() {
  var response = $.ajax({ type: "GET",
                          dataType: "text",
                          url: "index.php?git=gethash",
                          cache: false,
                          async: false
                        }).responseText;
  return response;
}

function githubData() {
  var result="";
  $.ajax({
      async: false,
      dataType: 'json',
      url: "https://api.github.com/repos/mescon/Muximux/commits",
      type: 'GET',
        success: function(data) {
          result = data;
          }

  });
  return result;
}

function checkVersion() {
  var json = githubData();
  var localversion = getLocalVersion();
  var compareURL = "https://github.com/mescon/Muximux/compare/" + getLocalVersion() + "..." + json[0].sha;
  var difference = 0;
  for (var i in json)
  {
    if(json[i].sha == localversion) {
      difference = i;
    }
  }
  var differenceDays = datediff(json[0].commit.author.date.substring(0,10));

  var upstreamInformation = { compareURL: compareURL,
                        differenceCommits: difference,
                        differenceDays: differenceDays,
                        latestVersion: json[0].sha,
                        localVersion: localversion }
  return upstreamInformation;
}

function viewChangelog() {
    var output="";
  $.ajax({
    url: "https://api.github.com/repos/mescon/Muximux/commits",
    //force to handle it as text
    dataType: "text",
      success: function(data) {

        var json = $.parseJSON(data);
        var status = "up to date!";
        if(checkVersion().differenceCommits < 0) {
            status = checkVersion().differenceCommits + " commits ahead!";
        }
        if(checkVersion().differenceCommits > 0) {
            status = checkVersion().differenceCommits + " commits behind!";
        }
        if(checkVersion().localVersion == "unknown") {
            status = "running an unknown version.<br/>To enable this functionality, please install Muximux by typing <code>git clone https://github.com/mescon/Muximux</code> in your terminal.<br/>Please read the full <a href=\"https://github.com/mescon/Muximux#setup\" target=\"_blank\">setup instructions</a>.";
        }

        output="<p>Your version is currently <strong>"+ status +"</strong><br/>";
        if(checkVersion().differenceCommits > 0) {
            output+= "The changes from your version to the latest version can be read <a href=\"" + checkVersion().compareURL + "\" target=\"_blank\">here</a>.</p>";
        }

        output+="<p>The latest update to Muximux was uploaded to Github " + checkVersion().differenceDays + " days ago.</p>";
        output+="<p>If you wan't to update, please do <code>git pull</code> in your terminal, or <a href='https://github.com/mescon/Muximux/archive/master.zip' target='_blank'>download the latest zip.</a></p><br/><h3>Changelog</h3><ul>";
        for (var i in json)
        {
          var shortCommitID = json[i].sha.substring(0,7);
          var shortComments = json[i].commit.message.substring(0,220).replace(/$/, "") + "...";
          var shortDate = json[i].commit.author.date.substring(0,10);

          output+="<li><pre>"+ shortDate +" <a href=\"" + json[i].html_url + "\">" + shortCommitID + "</a>:  " + shortComments + "</li></pre>";

        }
        output+= "</ul>";
        $('#changelog').html(output);
      }
  });
}