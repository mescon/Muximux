var branch = $("#branch-data").attr('data');
var commitURL = "https://api.github.com/repos/mescon/Muximux/commits?sha=" + branch;
var localversion = $("#sha-data").attr('data');
var cwd = $("#cwd-data").attr('data');
var phpini = $("#phpini-data").attr('data');
var gitdir = $("#gitdirectory-data").attr('data');
var title = $("#title-data").attr('data');
var branch = $("#branch-data").attr('data');
var secret = $("#secret").attr('data');
var jsonString;
var difference = 0;
var n = 15;
var differenceDays;
var json;


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
// drawer is just a boolean value as to whether or not we have the drawer.
// If we do, then we need to make the iframe taller than bar height.
function resizeIframe(drawer, isMobile) {
    if (drawer && !isMobile) {
        var newSize = $(window).height() - 5;
        $('iframe').css({
            'height': newSize + 'px'
        });
    } else {
        var newSize = $(window).height() - $('.cd-tabs-navigation').height();
        $('iframe').css({
            'height': newSize + 'px'
        });
    }
}
// From https://css-tricks.com/snippets/javascript/htmlentities-for-javascript/ - don't render tags retrieved from Github
function htmlEntities(str) {
    return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function dropDownFixPosition(button, dropdown) {
    var dropDownTop = button.offset().top + button.outerHeight();
    dropdown.css('top', dropDownTop + "px");
    dropdown.css('left', $(window).width() - $('.drop-nav').width() - button.offset().left + "px");
}

function settingsEventHandlers() {
    $('#sortable').sortable();
    // Anytime a radio box for default is changed it unchecks the others
    $('input[type=radio]').change(function() {
        $('input[type=radio]:checked').not(this).prop('checked', false);
    });
    // Event Handler for show/hide instructions
    $('#showInstructions').click(function() {
        $('#instructionsContainer').slideToggle(1000);
        if ($(this).html() == "<span class=\"fa fa-book\"></span> Show Guide") $(this).html('<span class=\"fa fa-book\"></span> Hide Guide');
        else $(this).html('<span class=\"fa fa-book\"></span> Show Guide');
    });
    // Event Handler for show/hide changelog
    $('#showChangelog').click(function() {
        $('#changelogContainer').slideToggle(1000);
        if ($(this).html() == "<span class=\"fa fa-github\"></span> Show Updates") {
            $(this).html('<span class=\"fa fa-github\"></span> Hide Updates');
            viewChangelog();
        }
        else $(this).html('<span class=\"fa fa-github\"></span> Show Updates');
    });
    // Event Handler for backup.ini show/hide button
    if ($('#backupContents').text() != "") {
        $('#topButtons').append('<a class="btn btn-primary" id="showBackup"><span class=\"fa fa-book\"></span> Show Backup INI</a>')
        $('#topButtons').css('width', '425px')
        $('#showBackup').click(function() {
            $('#backupiniContainer').slideToggle(1000);
            if ($(this).html() == "<span class=\"fa fa-book\"></span> Show Backup INI") $(this).html('<span class=\"fa fa-book\"></span> Hide Backup INI');
            else $(this).html('<span class=\"fa fa-book\"></span> Show Backup INI');
        });
    }
    // Remove all event handler
    $('#removeAll').click(function() {
        if (confirm('Are you sure?')) {
            var selectedEffect = "drop";
            var options = {};
            var time = 150;
            $(this).parents('form').children('#sortable').children().reverse().each(function() {
                var that = $(this);
                setTimeout(function() {
                    that.effect(selectedEffect, options, 150, removeCallback(that))
                }, time);
                time = time + 150;
            });
            $('#addApplication').click();
        }
    });
    // Remove sortable item button handler
    $('form').on('click', '.removeButton', function() {
        if (confirm('Are you sure?')) {
            var selectedEffect = "drop";
            var options = {};
            var remID = $(this).attr("id").split('-');
            $($(this).parents('.applicationContainer')).effect(selectedEffect, options, 500, removeCallback($(this).parents('.applicationContainer')));
            write_log('Removed application named ' + remID[1]);
        }
    });
    $('#removeBackup').click(function() {
        var secret = $("#secret").data()['data'];
        $.ajax({
            async: true,
            url: "muximux.php",
            type: 'GET',
            data: {
                remove: "backup",
                secret: secret
            },
            success: function(data) {
                if (data == "deleted");
                $('#backupiniContainer').toggle(1000);
                $('#showBackup').remove();
                $('#topButtons').css('width', '280px')
            }
        });
    });

    function removeCallback(selectedElement) {
        setTimeout(function() {
            $(selectedElement).remove();
        }, 1000);
    };
    // Fix for iconpicker. For some reason the arrow doesn't get disabled when it hits the minimum/maximum page number. This disables the button, so that it doesnt go into the negatives or pages above its max.
    $('body').on('click', '.btn-arrow', function(event) {
        event.preventDefault();
        if ($(this).hasClass('disabled')) $(this).attr('disabled', 'disabled');
        else $('.btn-arrow').removeAttr('disabled');
    });
    // Add new application button handler
    $('#addApplication').click(function() {
        // Generating a random number here. So that if the user adds more than one new application at a time the ids/classes and names don't match.
        var rand = Math.floor((Math.random() * 999999) + 1);
        $('#sortable').append(
		'<div class="applicationContainer newApp" id="' + rand + 'newApplication">' +
			'<span class="bars fa fa-bars"></span>' +
			'<div class="appdiv form-group">' +
				'<label for="' + rand + 'newApplication_-_name" class="col-xs-4 control-label right-label">Name: </label>' +
				'<div class="col-xs-7 col-md-8">' +
					'<input class="form-control form-control-sm appName ' + rand + 'newApplication_-_value" name="' + rand + 'newApplication_-_name" type="text" value="">' + 
				'</div>' +
			'</div>' + 
			'<div class="appdiv form-group">' +
				'<label for="' + rand + 'newApplication_-_value" class="col-xs-4 control-label right-label">URL: </label>' +
				'<div class="col-xs-7 col-md-8">' +
					'<input class="form-control form-control-sm ' + rand + 'newApplication_-_value" name="' + rand + 'newApplication_-_url" type="text" value="">' + 
				'</div>' +
			'</div><br>' + 
			'<div class="appdiv form-group">' +
				'<label for="' + rand + 'newApplication_-_icon" class="col-xs-4 control-label right-label">Icon: </label>' +
				'<div class="col-xs-7 col-md-5">' +
					'<button role="iconpicker" class="form-control form-control-sm iconpicker btn btn-default ' + rand + 'newApplication_-_icon" name="' + rand + 'newApplication_-_icon" id="' + rand + 'newApplication_-_iconPicker" data-rows="4" data-cols="6" data-search="true" data-search-text="Search..." data-iconset="muximux" data-placement="left">' + 
				'</div>' +
			'</div>' + 
			'<div class="appdiv form-group colorDiv">' +
				'<label for="' + rand + 'newApplication_-_color" class="col-xs-4 col-md-5 control-label color-label">Color: </label>' +
				'<div class="col-xs-7">' +
					'<input type="text" id="' + rand + 'newApplication_-_color" class="form-control form-control-sm appsColor ' + rand + 'newApplication_-_color" name="' + rand + 'newApplication_-_color" style="display:none;" value="">' + 
				'</div>' +
			'</div>' +
			'<div class="appdiv form-group">' +
				'<label for="' + rand + 'newApplication_-_enabled" class="col-xs-6 col-md-12 control-label col-form-label form-check-inline">Enabled: ' +
					'<input type="checkbox" class="form-check-input form-control ' + rand + 'newApplication_-_value" id="' + rand + 'newApplication_-_enabled" name="' + rand + 'newApplication_-_enabled" checked>' + 
				'</label>' +
			'</div>' +
			'<div class="appdiv form-group">' +
				'<label for="' + rand + 'newApplication_-_enabled" class="col-xs-6 col-md-12 control-label col-form-label form-check-inline">Landing: ' +
					'<input type="checkbox" class="form-check-input form-control ' + rand + 'newApplication_-_value" id="' + rand + 'newApplication_-_landingpage" name="' + rand + 'newApplication_-_landingpage">' + 
				'</label>' +
			'</div>' +
			'<div class="appdiv form-group">' +
				'<label for="' + rand + 'newApplication_-_dd" class="col-xs-6 col-md-12 control-label col-form-label form-check-inline">Dropdown: ' +
					'<input type="checkbox" class="form-check-input form-control ' + rand + 'newApplication_-_value" id="' + rand + 'newApplication_-_dd" name="' + rand + 'newApplication_-_dd">' + 
				'</label>' +
			'</div>' +
			'<div class="appdiv form-group">' +
				'<label for="' + rand + 'newApplication_-_default" class="col-xs-6 col-md-12 control-label col-form-label form-check-inline">Default: ' +
					'<input type="radio" class="form-check-input form-control ' + rand + 'newApplication_-_value" id="' + rand + 'newApplication_-_default" name="' + rand + 'newApplication_-_default">' + 
				'</label>' +
			'</div>' + 

			'<button type="button" class="form-control form-control-sm removeButton btn btn-danger btn-xs" value="Remove" id="remove_-_' + rand + 'newApplication">Remove<meta class="newAppRand" value="' + rand + '"></button>' +
			'<meta class="newAppRand" value="' + rand + '">' +
		'</div>');
        initIconPicker('#' + rand + 'newApplication_-_iconPicker');
		$('.' + rand + 'newApplication_-_color').spectrum({
		showInput: true,
		showPalette: true,
		preferredFormat: "hex",
		palette: [
        ["#000","#444","#666","#999","#ccc","#eee","#f3f3f3","#fff"],
        ["#f00","#f90","#ff0","#0f0","#0ff","#00f","#90f","#f0f"],
        ["#f4cccc","#fce5cd","#fff2cc","#d9ead3","#d0e0e3","#cfe2f3","#d9d2e9","#ead1dc"],
        ["#ea9999","#f9cb9c","#ffe599","#b6d7a8","#a2c4c9","#9fc5e8","#b4a7d6","#d5a6bd"],
        ["#e06666","#f6b26b","#ffd966","#93c47d","#76a5af","#6fa8dc","#8e7cc3","#c27ba0"],
        ["#c00","#e69138","#f1c232","#6aa84f","#45818e","#3d85c6","#674ea7","#a64d79"],
        ["#900","#b45f06","#bf9000","#38761d","#134f5c","#0b5394","#351c75","#741b47"],
        ["#600","#783f04","#7f6000","#274e13","#0c343d","#073763","#20124d","#4c1130"]
    ]
	});
		$('.sp-replacer').addClass(' form-control');
		$('.sp-replacer').addClass('form-control-sm');
    });
    // App Name Change/Addition handler
    $('form').on('focusout', '.appName', function() {
        if ($(this).val() != "") {
            $(this).parents('.applicationContainer').removeClass('newApp');
            var section = $(this).attr('was');
            if (section == undefined) {
                section = $(this).parents('.applicationContainer').children('.newAppRand').attr('value') + "newApplication";
                $(this).parents('applicationContainer').children('.newAppRand').remove();
            }
            var newSection = $(this).val().split(' ').join('_');
            $(this).attr('was', newSection);
            $('.' + section + '-value').each(function() {
                var split = $(this).attr('name').split('-');
                $(this).removeAttr('name').prop('name', newSection + "-" + split[1]).addClass(newSection + '-value').removeClass(section + '-value');
            });
            $('input[name="' + section + '-icon"]').prop('name', newSection + "-icon");
            $(this).parents('div.applicationContainer').attr('id', newSection);
        }
    });
    //On Submit handler
    var options = {
        url: 'muximux.php',
        type: 'post',
        success: showResponse
    };
    $('#settingsSubmit').click(function(event) {
        event.preventDefault();
        $('.newApp').remove(); //Remove any new app that isn't filled out.
        $('.checkbox,.radio').each(function() {
            if (!$(this).prop('checked')) {
                var name = $(this).attr('name');
                $('<input type="hidden" name="' + name + '" value="false">').appendTo($(this));
            }
        });
        $('.appName').removeAttr('disabled');
        $("form").ajaxSubmit(options);
    });
    $('#tabcolorCheckbox').click(function(event) {
        if ($(this).prop('checked')) {
            $('.colorDiv').toggle('slide', {direction: 'right'}, 200);            
        } else {
            $('.colorDiv').toggle('slide', {direction: 'right'}, 200);            
        }

    });
}
// Takes all the data we have to generate our changelog
function viewChangelog() {
    $('#changelog').html("");
    if ((!getCookie('hasJSON')) || (!sessionStorage['JSONData'])) {
        write_log('Refreshing commit data from github - ' + ($force ? "automatically triggered." : "manually triggered."));
        updateJson();
    } 
		json = JSON.parse(sessionStorage.getItem('JSONData'));
	
    $.getJSON(commitURL, function(result) {
        json = result;
        var compareURL = "https://github.com/mescon/Muximux/compare/" + localversion + "..." + json[0].sha;
        difference = 0;
        for (var i in json) {
            if (json[i].sha == localversion) {
                difference = i;
            }
        }
        differenceDays = datediff(json[0].commit.author.date.substring(0, 10));


    var status = "<strong>up to date!</strong>";
    if (difference > 0) {
        status = "<strong>" + difference + " commits behind!</strong>";
    }
    output = "<p>Your install is currently " + status + "<br/>";
    if (difference > 0) {
        output += "The changes from your version to the latest version can be read <a href=\"" + compareURL + "\" target=\"_blank\">here</a>.</p>";
    }
    output += "<p>Updates to your version of <a href='https://github.com/mescon/Muximux/' target='_blank'>Muximux</a> were uploaded to Github " + (differenceDays == 1 ? 'today' : differenceDays - 1 + (differenceDays == 2 ? ' day ago' : ' days ago') ) + ".</p>";
    output += "<div class='btn-group' role='group' aria-label='Buttons' id='topButtons'>";
    if (difference > 0) {
        output +="<a class='btn btn-primary' id='downloadUpdate'><span class='fa fa-arrow-circle-down'></span> Install Now</a>";
    }
    output +="<a class='btn btn-primary' id='refreshUpdate'><span class='fa fa-rotate-right'></span> Refresh Updates</a>" +
                    "</div>";
    if (difference > 0) {

        output += "<p>Or you can manually download <a href='https://github.com/mescon/Muximux/archive/" + branch + ".zip' target='_blank'>the latest zip here.</a></p>";
        output += "<h3>Changelog (" + branch + ")</h3><ul>";
        var i=0;
        do {
            var shortCommitID = json[i].sha.substring(0, 7);
            var shortComments = htmlEntities(json[i].commit.message.substring(0, 550).replace(/$/, "") + "...");
            var shortDate = json[i].commit.author.date.substring(0, 10);
            output += "<li><pre>" + shortDate + " <a href=\"" + json[i].html_url + "\">" + shortCommitID + "</a>:  " + shortComments + "</li></pre>";
            i++;
        } while (i != difference);
        output += "</ul>";
    }
    $('#changelog').html(output);
    $('#downloadUpdate').click(function(){
        downloadUpdate(json[0].sha);
    });
    $('#refreshUpdate').click(function(){
        refreshBranches();
        updateBox(true);
    });


    });
}

function refreshBranches() {
    $.ajax({
            async: true,
            url: "muximux.php",
            type: 'GET',
            data: {action: "branches", secret: secret },
            success: function (data) {
                viewChangelog();
            }

        });
}

//Init iconpickers
function initIconPicker(selectedItem) {
    $(selectedItem).iconpicker({
        align: 'center', // Only in div tag
        arrowClass: 'btn-danger',
        arrowPrevIconClass: 'glyphicon glyphicon-chevron-left',
        arrowNextIconClass: 'glyphicon glyphicon-chevron-right',
        cols: 10,
        footer: true,
        header: true,
        iconset: 'muximux',
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
// post-submit callback function
function showResponse(responseText, statusText) {
    if (responseText == 1 || statusText == "success") location.pathname = location.pathname;
    else alert("Error!!!-" + responseText);
}
// Calculates the amount of days since an update was commited on Github.
function datediff(latestDate) {
    var githubDate_ms = new Date(latestDate).getTime();
    var localDate_ms = new Date().getTime();
    var difference_ms = localDate_ms - githubDate_ms;
    return Math.round(difference_ms / 86400000);
}
// Gets the secret key that was generated on load. This AJAX call can not be async - other functions rely on this property to be set first.
function getSecret() {
    $.ajax({
        async: true,
        dataType: 'text',
        url: "muximux.php?get=secret",
        type: 'GET',
        success: function(data) {
            $('#secret').data({
                data: data
            });
        }
    });
}


function setCookie(cname, cvalue, exdays) {
    var d = new Date();
    d.setTime(d.getTime() + (exdays * 24 * 60 * 60 * 1000));
    var expires = "expires=" + d.toUTCString();
    document.cookie = cname + "=" + cvalue + "; " + expires;
}

function getCookie(cname) {
    var name = cname + "=";
    var ca = document.cookie.split(';');
    for (var i = 0; i < ca.length; i++) {
        var c = ca[i];
        while (c.charAt(0) == ' ') c = c.substring(1);
        if (c.indexOf(name) == 0) return c.substring(name.length, c.length);
    }
    return "";
}
// Set document title including title of the page as configured in settings.ini.php
function setTitle(title) {
    $(document).attr("title", title + " - " + $('#maintitle').attr('data'));
}
// Idea and implementation graciously borrowed from PlexPy (https://github.com/drzoidberg33/plexpy)
function updateBox($force) {
    if ((!getCookie('hasJSON')) || ($force === true) || (!sessionStorage['JSONData'])) {
        write_log('Refreshing commit data from github - ' + ($force ? "automatically triggered." : "manually triggered."));
        updateJson();
    } 
	json = JSON.parse(sessionStorage.getItem('JSONData'));
	
    var compareURL = "https://github.com/mescon/Muximux/compare/" + localversion + "..." + json[0].sha;
    var difference = 0;
    for (var i in json) {
        if (json[i].sha == localversion) {
            difference = i;
        }
    }
    var differenceDays = datediff(json[0].commit.author.date.substring(0, 10));
    var updateCheck;
    if (difference) {
        clearInterval(updateCheck);
        if (difference > 0) {

            if (!getCookie('updateDismiss')) {
                $('#updateContainer').html("<button type='button' id='updateDismiss' class='close pull-right'>&times;</button>" +
                "<span>You are currently <strong>" + difference + "</strong> "+ ((difference > 1) ? 'commits' : 'commit')+" behind!<br/>" +
                "See <a href='" + compareURL + "' target='_blank'>changelog</a> or <div id='downloadModal'><code>click here</code></div> to install now.</span>");
                $('#updateContainer').fadeIn("slow");
                $('#downloadModal').click(function(){
                    downloadUpdate(json[0].sha);
                });
            }
        }
        $('#updateDismiss').click(function() {
            $('#updateContainer').fadeOut('slow');
            // Set cookie to remember dismiss decision for 1 hour.
            setCookie('updateDismiss', 'true', 1 / 24);
            write_log('Update notification dismissed for one hour.');
        });
    }
}

function updateJson() {
    $.getJSON(commitURL, function(result) {
            jsonString = JSON.stringify(result);
            sessionStorage.setItem('JSONData',jsonString);
            setCookie('hasJSON', 'true', 0.00694444);
        });
		
}

function scaleContent(content, scale) {
    var newWidth = $(window).width() / scale;
    var newHeight = (($(window).height() / scale) - ($('nav').height() / scale));
    $('.cd-tabs-content').find('li[data-content="' + content + '"]').children('iframe').css({
        '-ms-transform': 'scale(' + scale + ')',
        '-moz-transform': 'scale(' + scale + ')',
        '-o-transform': 'scale(' + scale + ')',
        '-webkit-transform': 'scale(' + scale + ')',
        'transform': 'scale(' + scale + ')',
        'height': '' + newHeight + 'px',
        'width': '' + newWidth + 'px',
    });
}

function downloadUpdate($sha) {
    if (confirm('Would you like to download and install updates now?')) {
        $.ajax({
            async: true,
            url: "muximux.php",
            type: 'GET',
			dataType:'json',
            data: {action: "update", secret: secret, sha: $sha},
        })
		.done(function(res) {
			setStatus('Update installed successfully!',true);
			delete_cookie('hasJSON');
			sessionStorage.removeItem('JSONData');
			n = 15;
			var tm = setInterval(reloadTimer,1000);

		})
		.fail(function(res) {
			response=JSON.parse(res["responseText"]);
			setStatus('INSTALL FAILED: ' + response["message"],false);
		});

    } 
}

// A little countdown function to reload and tell the user why
function reloadTimer() {
	n--;
	$('#countBox').html("Page will be reloaded in " + n + " seconds.");
	if(n == 0){
		location.reload();
		clearInterval(tm);
	}
}

//delete a cooke
function delete_cookie(name) {
	document.cookie = name + '=; expires=Thu, 01 Jan 1970 00:00:01 GMT;';
}
 
//writes to log
function write_log($text,$lvl) {

    $.ajax({
        async: true,
        url: "muximux.php",
        type: 'GET',
        data: {action: "writeLog", secret: secret, msg: $text,lvl: $lvl},
    });

}

function refresh_log() {
    var secret = $("#secret").attr('data');
        $.ajax({
            async: true,
            dataType: 'text',
            url: "muximux.php?secret=" + secret + "&action=log",
            type: 'GET',
            success: function(html) {
                $('#logContainer').replaceWith(html);
                $('#logContainer').slideToggle();
            }
        });
}

function setStatus(message,showcounter) {
    $('#updateContainer').hide();
    $('#updateContainer').html("<button type=\"button\" id=\"updateClose\" class=\"close pull-right\">&times;</button>" +
    "<span>" + message + "<br/><p id='countBox'</p></span>");
    $('#updateContainer').fadeIn("slow");
    $('#updateClose').click(function() {
        $('#updateContainer').fadeOut('slow');
    });
}

// Find apps with a scale setting that is more or less than 100%, then re-scale it to the desired setting using scaleContent(selector, scale)
function scaleFrames() {
    $('.cd-tabs-content').find('li').each(function(value) {
        content = $(this).attr('data-content');
        scale = $(this).attr('data-scale');
        // Mark the scale we are currently using, on the settings modal
        $('#' + content + '-scale option[value="' + scale + '"]').attr('selected', 'selected');
        // If scale is set to something other than 1, rescale it to what's stored in settings.
        if (scale !== "1") {
            scaleContent(content, scale);
        }
    })
}
// Like the name implies, it changes the favicon to whatever
// url is passed to it
function changeFavicon(src) {
    document.head = document.head || document.getElementsByTagName('head')[0];
    var link = document.createElement('link'),
        oldLink = document.getElementById('dynamic-favicon');
    link.id = 'dynamic-favicon';
    link.rel = 'shortcut icon';
    link.href = src;
    if (oldLink) {
        document.head.removeChild(oldLink);
    }
    document.head.appendChild(link);
}

// Wrap a html-encoded string in a div (in-memory) and read it back, unencoded.
function htmlDecode(value) {
    return $('<div/>').html(value).text();
}

// This fetches the browser-appropriate box-shadow value so we can set it
function getsupportedprop(proparray) {
    var root = document.documentElement //reference root element of document
    for (var i = 0; i < proparray.length; i++) { //loop through possible properties
        if (proparray[i] in root.style) { //if property exists on element (value will be string, empty string if not set)
            return proparray[i] //return that string
        }
    }
}


// Shhh, we just won't mention this is here for now
function setupFeed(url) {
	
$('#feed').rssfeed(url, {
    ssl: true,
    limit: 20,
    showerror: true,
    errormsg: '',
    tags: true,
    date: true,
    dateformat: 'spellmonth',
    titletag: 'h4',
    content: true,
    image: true,
    snippet: true,
    snippetlimit: 120,
    linktarget: '_blank'
}, function () {
    // optional callback function
});

}