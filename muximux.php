<?php
/*
* DO NOT CHANGE THIS FILE!
*/
defined("CONFIG") ? null : define('CONFIG', 'settings.ini.php');
defined("CONFIGEXAMPLE") ? null : define('CONFIGEXAMPLE', 'settings.ini.php-example');
defined("SECRET") ? null : define('SECRET', 'secret.txt');
require dirname(__FILE__) . '/vendor/autoload.php';

// Check if this is an old installation that needs upgrading.
if (file_exists('config.ini.php')) {
    copy('config.ini.php', 'backup.ini.php');
    unlink('config.ini.php');
    $upgrade = true;
    write_log('Converting configuration file from previous Muximux installation.','D');
} else {
    $upgrade = false;
}

// Check if this is our first run and do some things.
if(!file_exists(CONFIG)){
    copy(CONFIGEXAMPLE, CONFIG);
    checksetSHA();
}

// First what we're gonna do - save or read
if (sizeof($_POST) > 0) {
    if("" == trim($_POST['username'])){
        write_ini();
    }
} 

// Check if we can open a file.
function openFile($file, $mode) {
    if ((file_exists($file) && (!is_writable(dirname($file)) || !is_writable($file))) || !is_writable(dirname($file))) { // If file exists, check both file and directory writeable, else check that the directory is writeable.
        printf('Either the file %s and/or it\'s parent directory is not writable by the PHP process. Check the permissions & ownership and try again.', $file);
    write_log('Error writing to file ' . $file,'E');
        if (PHP_SHLIB_SUFFIX === "so") { //Check for POSIX systems.
            printf("<br>Current permission mode of %s: %d", $file, decoct(fileperms($file) & 0777));
            printf("<br>Current owner of %s: %s", $file, posix_getpwuid(fileowner($file))['name']);
            printf("<br>Refer to the README on instructions how to change permissions on the aforementioned files.");
        } else if (PHP_SHLIB_SUFFIX === "dll") {
            printf("<br>Detected Windows system, refer to guides on how to set appropriate permissions."); //Can't get fileowner in a trivial manner.
        }
        exit;
    }
    return fopen($file, $mode);
}

// Create a secret for communication to the server
function createSecret() {
    $text = uniqid("muximux-", true);
    $file = openFile(SECRET, "w");
    fwrite($file, $text);
    fclose($file);
    return $text;
}

// Save our settings on submit
function write_ini()
{
    $config = new Config_Lite(CONFIG);
    $oldHash = getPassHash();
    $oldBranch = getBranch();
    $terminate = false;
    $authentication = $config->getBool('general','authentication',false);
    unlink(CONFIG);
    $config = new Config_Lite(CONFIG);
    foreach ($_POST as $parameter => $value) {
        $splitParameter = explode('_-_', $parameter);
        $value = (($value == "on") ? "true" : $value );
        if ($splitParameter[1] == "password") {
            if ($value != $oldHash) {
                write_log('Successfully updated password.','I');
                $value = password_hash($value, PASSWORD_BCRYPT);
                $terminate = true;
            }
        }
        if ($splitParameter[1] == "authentication") {
            if ($value != $authentication) {
                $terminate = true;
            }
        }
        if ($splitParameter[1] == "branch") {
            if ($value != $oldBranch) {
                $config->set('settings','branch_changed',true);
                $config->set('settings','sha','00');
            } else {
                $config->set('settings','branch_changed',false);
            }
        }
        $config->set($splitParameter[0], $splitParameter[1], $value);
    }
    // save object to file
    saveConfig($config);
    if ($terminate) {
        session_start();
        session_destroy();
    }
}

// Parse settings.php and create the Muximux elements
function parse_ini()
{
    $config = new Config_Lite(CONFIG);
    checksetSHA();
    
    fetchBranches(false);
    $branchArray = getBranches();
    $branchList = "";
        
    $css = './css/theme/' . getTheme() . '.css';
    $tabColorEnabled = $config->getBool('general', 'tabcolor', false);
    $enableDropDown = $config->getBool('general', 'enabledropdown', false);
    $updatePopup = $config->getBool('general', 'updatepopup', false);
    $mobileOverride = $config->getBool('general', 'mobileoverride', false);
    $cssColor = ((parseCSS($css,'.colorgrab','color') != false) ? parseCSS($css,'.colorgrab','color') : '#FFFFFF');
    $themeColor = $config->get('general','color',$cssColor);
    $autoHide = $config->getBool('general', 'autohide', false);
    $splashScreen = $config->getBool('general', 'splashscreen', false);
    $userName = $config->get('general', 'userNameInput', 'admin');
    $passHash = $config->get('general', 'password', 'Muximux');
    $authentication = $config->getBool('general', 'authentication', false);

    foreach ($branchArray as $branchName => $shaSum ) {
        $branchList .= "
                                <option value='".$branchName."' ".(($myBranch == $branchName) ? 'selected' : '' ).">". $branchName ."</option>";
    }
    $title = $config->get('general', 'title', 'Muximux - Application Management Console');
    $pageOutput = "
                    <form>
                        <div class='applicationContainer generalContainer' style='cursor:default;'>
                        <h2>Settings</h2>
                        <div>
                            <label for='titleInput'>Main Title: </label>
                            <input id='titleInput' type='text' class='general_-_value' name='general_-_title' value='" . $title . "'>
                        </div>
                        <div>
                            <label for='theme'>Theme: </label>
                            <select id='theme' class='general_-_value' name='general_-_theme'>".
                                listThemes() ."
                            </select>
                        </div>
                        <div>
                            <label for='branch'>Branch tracking: </label>
                            <select id='branch' name='general_-_branch'>".
                                $branchList ."
                            </select>
                        </div>
                        <br>
                        <div>
			<div>
                            <label for='splashscreenCheckbox'>Start with splash screen:</label>
                            <input id='splashscreenCheckbox' class='general_-_value' name='general_-_splashscreen' type='checkbox' ".($splashScreen ? 'checked' : '') .">
                        </div>
                        <div>
                            <label for='updatepopupCheckbox'>Enable update poups:</label>
                            <input id='updatepopupCheckbox' class='general_-_value' name='general_-_updatepopup' type='checkbox' ".($updatePopup ? 'checked' : '') .">
                        </div>

                        <div>
                            <label for='mobileoverrideCheckbox'>Enable mobile override:</label>
                                <input id='mobileoverrideCheckbox' class='general_-_value' name='general_-_mobileoverride' type='checkbox' ".($mobileOverride ? 'checked' : '').">
                        </div><br>
                        <div class='generalColor'>
                            <label for='general_-_color'>Color: </label>
                            <input type='color' id='general_-_default' class='generalColor general_-_color' value='".$themeColor."' name='general_-_color'>
                        </div>
                        <div>
                            <label for='tabcolorCheckbox'>Enable per-tab colors:</label>
                            <input id='tabcolorCheckbox' class='general_-_value' name='general_-_tabcolor' type='checkbox' " . ($tabColorEnabled ? 'checked' : '').">
                        </div>
                        <div>
                            <label for='autohideCheckbox'>Enable auto-hide:</label>
                            <input id='autohideCheckbox' class='general_-_value' name='general_-_autohide' type='checkbox' ".($autoHide ? 'checked' : '').">
                        </div>
                        <div>
                            <label for='authenticationCheckbox'>Enable authentication:</label>
                            <input id='authenticationCheckbox' class='general_-_value' name='general_-_authentication' type='checkbox' ".($authentication ? 'checked' : '').">
                        </div><br>
                        <div class='inputdiv'>
                            <div class='userinput'>
                                <label for='userName'>Username: </label><input id='userNameInput' type='text' class='general_-_value userinput' name='general_-_userNameInput' value='" . $userName . "'>
                            </div>
                            <div class='userinput'>
                                <label for='password'>Password: </label><input id='passwordInput' type='password' autocomplete='new-password' class='general_-_value userinput' name='general_-_password' value='" . $passHash . "'>
                            </div>
                        </div>
                    </div>
                </div>
                <input type='hidden' class='settings_-_value' name='settings_-_sha' value='".$mySha."'>
                <input type='hidden' class='settings_-_value' name='settings_-_enabled' value='true'>
                <input type='hidden' class='settings_-_value' name='settings_-_default' value='false'>
                <input type='hidden' class='settings_-_value' name='settings_-_name' value='Settings'>
                <input type='hidden' class='settings_-_value' name='settings_-_url' value='muximux.php'>
                <input type='hidden' class='settings_-_value' name='settings_-_landingpage' value='false'>
                <input type='hidden' class='settings_-_value' name='settings_-_icon' value='fa-cog'>
                <input type='hidden' class='settings_-_value' name='settings_-_dd' value='true'>
                <div id='sortable'>";
    foreach ($config as $section => $name) {
        if (is_array($name) && $section != "settings" && $section != "general") {
            $name = $config->get($section, 'name', '');
            $url = $config->get($section, 'url', 'http://www.plex.com');
            $color = $config->get($section, 'color', '#000');
            $icon = $config->get($section, 'icon', '');
            $scale = $config->get($section, 'scale', '');
            $default = $config->getBool($section, 'default', false);
            $enabled = $config->getBool($section, 'enabled', true);
            $landingpage = $config->getBool($section, 'landingpage', true);
            $dd = $config->getBool($section, 'dd', true);
            $scaleRange = "0";
            $scaleRange = buildScale($scale);
            $pageOutput .= "
                    <div class='applicationContainer' id='" . $section . "'>
                        <span class='bars fa fa-bars'></span>
                        <div>
                            <label for='" . $section . "_-_name' >Name: </label>
                            <input class='appName " . $section . "_-_value' was='" . $section . "' name='" . $section . "_-_name' type='text' value='" . $name . "'>
                        </div>
                        <div>
                            <label for='" . $section . "_-_url' >URL: </label>
                            <input class='" . $section . "_-_value' name='" . $section . "_-_url' type='text' value='" . $url . "'>
                        </div>
                        <div>
                            <label for='" . $section . "_-_enabled' >Enabled: </label>
                            <input type='checkbox' class='checkbox " . $section . "_-_value' id='" . $section . "_-_enabled' name='" . $section . "_-_enabled'".($enabled ? 'checked' : '') .">
                        </div>
                        <div>
                            <label for='" . $section . "_-_landingpage' >Landing page: </label>
                            <input type='checkbox' class='checkbox " . $section . "_-_value' id='" . $section . "_-_landingpage' name='" . $section . "_-_landingpage'".($landingpage ? 'checked' : '') .">
                        </div>
                        <div>
                            <label for='" . $section . "_-_dd' >Put in dropdown: </label>
                            <input type='checkbox' class='checkbox " . $section . "_-_value' id='" . $section . "_-_dd' name='" . $section . "_-_dd'".($dd ? 'checked' : '') .">
                        </div>
                        <div>
                            <label for='" . $section . "_-_default' >Default: </label>
                            <input type='radio' class='radio " . $section . "_-_value' id='" . $section . "_-_default' name='" . $section . "_-_default'".($default ? 'checked' : '') .">
                        </div>
                        <br>
                        <div class='appsColor'>
                            <label for='" . $section . "_-_color'>Color: </label>
                            <input type='color' id='" . $section . "_-_color' class='appsColor " . $section . "_-_color' value='" . $color . "' name='" . $section . "_-_color'>
                        </div>
                        <div>
                            <label for='" . $section . "_-_icon' >Icon: </label>
                            <button role=\"iconpicker\" class=\"iconpicker btn btn-default\" name='" . $section . "_-_icon' data-rows=\"4\" data-cols=\"6\" data-search=\"true\" data-search-text=\"Search...\" data-iconset=\"fontawesome\" data-placement=\"left\" data-icon=\"" . $icon . "\">
                            </button>
                        </div>
                        <div style=\"margin-left:5px;\">
                            <label for='" . $section . "_-_scale'>Zoom: </label>
                            <select id='" . $section . "_-_scale' name='" . $section . "_-_scale'>". $scaleRange ."
                            </select>
                        </div>
                        <button type='button' class='removeButton btn btn-danger btn-xs' value='Remove' id='remove-" . $section . "'>Remove</button>
                    </div>";
        }
    }
    $pageOutput .= "
                </div>
                <div class='text-center' style='margin-top: 15px;'>
                    <div class='btn-group' role='group' aria-label='Buttons'>
                        <a class='btn btn-primary btn-md' id='addApplication'><span class='fa fa-plus'></span> Add new</a>
                        <a class='btn btn-danger btn-md' id='removeAll'><span class='fa fa-trash'></span> Remove all</a>
                    </div>
                </div>
            </form>";
    return $pageOutput;
}

// Generate our splash screen contents (basically a very little version of parse_ini).
function splashScreen() {
	$config = new Config_Lite(CONFIG);
   $splash = "";
    
    foreach ($config as $keyname => $section) {
		if (($keyname != "general") && ($keyname != "settings")) {
			$splash .= "
									<div class='btnWrap'>
										<div class='well splashBtn' data-content=\"" . $keyname . "\">
											<a class='panel-heading' data-title=\"" . $section["name"] . "\">
												<br><i class='fa fa-5x " . $section["icon"] . "' style='color:".$section["color"]."'></i><br>
												<p style='color:#ddd'>".$section["name"]."</p>
											</a>
										</div>
									</div>";
		}
	}
	return $splash;
}

// Generate the contents of the log
function log_contents() {
    $out = '<ul>
                <div id="logContainer">
    ';
    $filename = 'muximux.log';
	$file = file($filename);
	$file = array_reverse($file);
	foreach($file as $line){
		$lvl = substr($line,0,1);
        if ($lvl === 'E') {
            $color = 'alert alert-danger';
        }
        if ($lvl === 'D') {
            $color = 'alert alert-warning';
        }
        if ($lvl === 'I') {
            $color = 'alert alert-success';
		}
        $out .='
                        <li class="logLine '.$color.'">'.
                            substr($line,2).'
                        </li>';

        
        
    }
    $out .= '</div>
            </ul>
    ';
    return $out;
}


// Check if the user changes tracking branch, which will change the SHA and trigger an update notification
function checkBranchChanged() {
    $config = new Config_Lite(CONFIG);
    if ($config->getBool('settings', 'branch_changed', false)) {
        $config->set("settings","sha","0");
        $config->set("settings","branch_changed",false);
        saveConfig($config);
        return true;
    } else {
        return false;
    }
}

// Build a custom scale using our set value, show it as selected
function buildScale($selectValue)
{
    $f=10;
    $scaleRange = "";
    while($f<251) {
        $pra = $f / 100;
        $scaleRange .= "
                                <option value='" . $pra ."' ".(($pra == $selectValue ? 'selected' : '')).">". $f ."%</option>";
        $f++;

    }
    return $scaleRange;
}

// Quickie to get the theme from settings
function getTheme()
{
    $config = new Config_Lite(CONFIG);
    $item = $config->get('general', 'theme', 'Classic');
    return $item;
}

// List all available themes in directory
function listThemes() {
    $dir    = './css/theme';
    $themelist ="";
    $themes = scandir($dir);
    $themeCurrent = getTheme();
    foreach($themes as $value){
        $splitName = explode('.', $value);
		if  (!empty($splitName[0])) {
			$name = ucfirst($splitName[0])
            $themelist .="
                                <option value='".$name."' ".(($name == getTheme()) ? 'selected' : '').">".$name."</option>";
        }
    }
    return $themelist;
}

// Build the contents of our menu
function menuItems() {
    $config = new Config_Lite(CONFIG);
    $standardmenu = "";
    $dropdownmenu = "";
    $int = 0;
    foreach ($config as $keyname => $section) {
        if (($keyname == "general")) {
            $autohide = $config->getBool('general', 'autohide', true);
            $enabledropdown = $config->getBool('settings', 'enabledropdown', true);
            $mobileoverride = $config->getBool('general', 'mobileoverride', false);
            $authentication = $config->getBool('general', 'authentication', false);
        } else {
            $dropdown = $config->getBool($keyname, 'dd', false);
            $enabled = $config->getBool($keyname, 'enabled', true);
            $default = $config->getBool($keyname, 'default', true);
        if ($enabled && !$dropdown) {
                $standardmenu .= "
                    <li class='cd-tab' data-index='".$int."'>
                        <a data-content='" . $keyname . "' data-title='" . $section["name"] . "' data-color='" . $section["color"] . "' class='".($default ? 'selected' : '')."'>
                            <span class='fa " . $section["icon"] . " fa-lg'></span> " . $section["name"] . "
                        </a>
                    </li>";
            $int++;
        }

        if ($dropdown && $enabled && $section['name'] == "Settings") {
            $dropdownmenu .= "
                    <li>
                        <a data-toggle=\"modal\" data-target=\"#settingsModal\" data-title=\"Settings\">
                            <span class=\"fa " . $section["icon"] . "\"></span> " . $section["name"] . "
                        </a>
                    </li>
                    <li>
                        <a data-toggle=\"modal\" data-target=\"#logModal\" data-title=\"Log Viewer\">
                            <span class=\"fa fa-file-text-o\"></span> Log
                        </a>
                    </li>
                    ";
        } else if ($dropdown && $enabled) {
            $dropdownmenu .= "
                    <li>
                        <a data-content=\"" . $keyname . "\" data-title=\"" . $section["name"] . "\">
                            <span class=\"fa " . $section["icon"] . "\"></span> " . $section["name"] . "
                        </a>
                    </li>";
        } else {
            $dropdownmenu .= "";
        }
    }
}

    $moButton = "
                <li class='navbtn ".(($mobileoverride == "true") ? '' : 'hidden')."'>
                    <a id=\"override\" title=\"Click this button to disable mobile scaling on tablets or other large-resolution devices.\">
                        <span class=\"fa fa-mobile fa-lg\"></span>
                    </a>
                </li>
    ";
    $outButton = "
                <li class='navbtn ".(($authentication == "true") ? '' : 'hidden')."'>
                    <a id='logout' title='Click this button to log out of Muximux.'>
                        <span class=\"fa fa-sign-out fa-lg\"></span>
                    </a>
                </li>
    ";


    $drawerdiv .= "<div class='cd-tabs-bar ".(($autohide == "true")? 'drawer' : '')."'>";

    if ($enabledropdown == "true") {
        $item = $drawerdiv . "
            <ul class=\"main-nav\">" .
            $moButton ."
                <li class='navbtn'>
                    <a id=\"reload\" title=\"Double click your app in the menu, or press this button to refresh the current app.\">
                        <span class=\"fa fa-refresh fa-lg\"></span>
                    </a>
                </li>".
                $outButton ."
                <li class='dd navbtn'>
                    <a id=\"hamburger\">
                        <span class=\"fa fa-bars fa-lg\"></span>
                    </a>
                    <ul class=\"drop-nav\">" .
                                $dropdownmenu ."
                    </ul>
                </li>
            </ul>
            <ul class=\"cd-tabs-navigation\">
                <nav>" .
                    $standardmenu ."
                </nav>
            </ul>
        </div>
        ";
    } else {
        $item =  $drawerdiv . "
            <ul class=\"main-nav\">" .
            $moButton ."
                <li class='cd-tab navbtn'>
                    <a id=\"reload\" title=\"Double click your app in the menu, or press this button to refresh the current app.\">
                        <span class=\"fa fa-refresh fa-lg\"></span>
                    </a>
                </li>
                <li class='cd-tab navbtn'>
                    <a id=\"log\" data-toggle=\"modal\" data-target=\"#logModal\" data-title=\"Log Fiewer\">
                        <span class=\"fa fa-file-text-o fa-lg\"></span>
                    </a>
                </li>
            </ul>
            <ul class=\"cd-tabs-navigation\">
                <nav>" .
                    $standardmenu . "
                </nav>
            </ul>
            ";
    }
    return $item;
}

// Quickie fetch the main title
function getTitle() {
    $config = new Config_Lite(CONFIG);
    $item = $config->get('general', 'title', 'Muximux - Application Management Console');
    return $item;

}
// Quickie fetch of the current selected branch
function getBranch() {
    $config = new Config_Lite(CONFIG);
    $branch = $config->get('general', 'branch', 'master');
    return $branch;
}

// Reads for "branches" from settings.  If not found, fetches list from github, saves, parses, and returns
function getBranches() {
    $config = new Config_Lite(CONFIG);
    $branches = [];
    $branches = $config->get('settings', 'branches',$branches);
    if ($branches == []) {
        fetchBranches(true);
    } else {
        $branches = $config->get('settings', 'branches');
    }
    return $branches;
}
// Fetch a list of branches from github, along with their current SHA
function fetchBranches($skip) {
    $config = new Config_Lite(CONFIG);
    $last = $config->get('settings', 'last_check', "0");
    if ((time() >= $last + 3600) || $skip) { // Check to make sure we haven't checked in an hour or so, to avoid making GitHub mad
        if (time() >= $last + 3600) {
            write_log('Refreshing branches from github - automatically triggered.');
        } else {
            write_log('Refreshing branches from github - manually triggered.');
        }
        $url = 'https://api.github.com/repos/mescon/Muximux/branches';
            $options = array(
          'http'=>array(
            'method'=>"GET",
            'header'=>"Accept-language: en\r\n" .
                      "User-Agent: Mozilla/5.0 (iPad; U; CPU OS 3_2 like Mac OS X; en-us) AppleWebKit/531.21.10 (KHTML, like Gecko) Version/4.0.4 Mobile/7B334b Safari/531.21.102011-10-16 20:23:10\r\n" // i.e. An iPad
          )
        );

        $context = stream_context_create($options);
        $json = file_get_contents($url,false,$context);
        if ($json == false) {
            write_log('Error fetching JSON from Github.','E');
            $result = false;
        } else {
            $array = json_decode($json,true);
            $i = 0;
            $names = array();
            $shas = array();
            foreach ($array as $value) {
                foreach ($value as $key => $value2) {
                    if ($key == "name") {
                            array_push($names,$value2);
                    } else {
                        foreach ($value2 as $key2 => $value3) {
                            if ($key2 == "sha" ) {
                                $shaVal = $value3;
                                array_push($shas,$value3);
                            }
                        }
                    }
                }
            }
            $outP = array_combine($names,$shas);
            $config ->set('settings','branches',$outP);
            $config ->set('settings','last_check',time());
            saveConfig($config);
            $result = true;
        }

    } else {
        $result = false;
    }
    return $result;

}

// Echos php information to the java console
function console_log( $data ) {
  $output  = "<script>console.log( 'PHP debugger: ";
  $output .= json_encode(print_r($data, true));
  $output .= "' );</script>";
  echo $output;
}

// This checks whether we have a SHA, and if not, whether we are using git or zip updates and stores
// the data accordingly
function checksetSHA() {
    $config = new Config_Lite(CONFIG);
	$shaIn = $config->get('settings','sha','0');
	$branchIn = getBranch();
	$shaCheck = (bool) preg_match('/^[0-9a-f]{40}$/i', $shaIn);
	$git = can_git();
	if ($git !== false) {
		$shaOut = exec('git rev-parse HEAD');
		$branchOut = exec('git rev-parse --abbrev-ref HEAD');
	} else {
		if (!$shaCheck) {
			$branchArray = getBranches();
			$branchOut = $branchIn();
			foreach ($branchArray as $branchName => $shaVal) {
				if ($branchName==$branchOut) {
					$shaOut = $shaVal;
				}
			}
		} 
	}
	$changed = false;
	if ($branchIn != $branchOut) {
		$config->set('settings', 'branch', $branchOut);
		$changed = true;
	}
	if ($shaIn != $shaOut) {
		$config->set('settings', 'sha', $shaOut);
		$changed = true;
	}
	if ($changed) {
		saveConfig($config);
        
	}
}

// Read SHA from settings and return it's value.
function getSHA() {
    $config = new Config_Lite(CONFIG);
    $item = $config->get('settings', 'sha', '00');
    return $item;
}

// Retrieve password hash from settings and return it for "stuff".
function getPassHash() {
    $config = new Config_Lite(CONFIG);
    $item = $config->get('general', 'password', 'foo');
    return $item;
}

// This little gem helps us replace a whome bunch of AJAX calls by sorting out the info and
// writing it to meta-tags at the bottom of the page.  Might want to look at calling this via one AJAX call.
function metaTags() {
    $config = new Config_Lite(CONFIG);
    $standardmenu = "";
    $dropdownmenu = "";
    $authentication = $config->getBool('general', 'authentication', false);
    $autohide = var_export($config->getBool('general', 'autohide', false),true);
    $greeting = $config->get('general', 'greeting', 'Hello.');
    $branch = $config->get('general', 'branch', 'master');
    $branchUrl = "https://api.github.com/repos/mescon/Muximux/commits?sha=" . $branch;
    $popupdate = var_export($config->getBool('general', 'updatepopup', true),true);
    $enabledropdown = var_export($config->getBool('settings', 'enabledropdown', true),true);
    $maintitle = $config->get('general', 'title', 'Muximux');
    $tabcolor = var_export($config->getBool('general', 'tabcolor', false),true);
    $splashScreen = var_export($config->getBool('general', 'splashscreen', false),true);
    $css = './css/theme/' . getTheme() . '.css';
    $cssColor = ((parseCSS($css,'.colorgrab','color') != false) ? parseCSS($css,'.colorgrab','color') : '#FFFFFF');
    $themeColor = $config->get('general','color',$cssColor);
    $inipath = php_ini_loaded_file();
        if ($inipath) {
            $inipath;
        } else {
            $inipath = "php.ini";
        }
    $created = filectime(CONFIG);
	$branchChanged = (checkBranchChanged() ? 'true' : 'false');
    $secret = file_get_contents(SECRET);
$tags = "
    <meta id='dropdown-data' data='".$enabledropdown."'>
    <meta id='branch-data' data='". $branch . "'>
    <meta id='branch-changed' data='". $branchChanged . "'>
    <meta id='popupdate' data='". $popupdate . "'>
    <meta id='drawer' data='". $autohide . "'>
    <meta id='tabcolor' data='". $tabcolor . "'>
    <meta id='maintitle' data='". $maintitle . "'>
    <meta id='gitdirectory-data' data='". $gitdir . "'>
    <meta id='cwd-data' data='". getcwd() . "'>
    <meta id='phpini-data' data='". $inipath . "'>
    <meta id='title-data' data='". $maintitle . "'>
    <meta id='created-data' data='". $created . "'>
    <meta id='sha-data' data='". getSHA() . "'>
    <meta id='secret' data='". $secret . "'>
    <meta id='themeColor-data' data='". $themeColor . "'>
    <meta id='splashScreen-data' data='". $splashScreen . "'>
    <meta id='authentication-data' data='". $authentication . "'>
";
    return $tags;
}

// Set up the actual iFrame contents, as the name implies.
function frameContent() {
    $config = new Config_Lite(CONFIG);
    if (empty($item)) $item = '';
    foreach ($config as $keyname => $section) {
    $landingpage = $config->getBool($keyname,'landingpage',false);
    $enabled = $config->getBool($keyname,'enabled',true);
    $default = $config->getBool($keyname,'default',false);
    $scale = $config->get($keyname,'scale',1);
    $url = $section["url"];
        $url=($landingpage ? "?landing=" . $keyname: $url);

        if ($enabled) {
            if (!empty($section["default"]) && !($section["default"] == "false") && ($section["default"] == "true")) {
                $item .= "
            <li data-content=\"" . $keyname . "\" data-scale=\"" . $section["scale"] ."\" class=\"selected\">
                <iframe sandbox=\"allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-popups allow-modals allow-top-navigation\"
                allowfullscreen=\"true\" webkitallowfullscreen=\"true\" mozallowfullscreen=\"true\" scrolling=\"auto\" data-title=\"" . $section["name"] . "\" src=\"" . $url . "\"></iframe>
            </li>";
            } else {
                $item .= "
                <li data-content=\"" . $keyname . "\" data-scale=\"" . $section["scale"] ."\">
                    <iframe sandbox=\"allow-forms allow-same-origin allow-pointer-lock allow-scripts allow-popups allow-modals allow-top-navigation\"
                    allowfullscreen=\"true\" webkitallowfullscreen=\"true\" mozallowfullscreen=\"true\" scrolling=\"auto\" data-title=\"" . $section["name"] . "\" data-src=\"" . $url . "\"></iframe>
                </li>
                ";
            }
        }
    }
    return $item;
}
// Build a landing page.
function landingPage($keyname) {
    $config = new Config_Lite(CONFIG);
    $item = "
    <html lang=\"en\">
    <head>
    <title>" . $config->get($keyname, 'name') . "</title>
    <link rel=\"stylesheet\" href=\"css/landing.css\">
    </head>
    <body>
    <div class=\"login\">
        <div class=\"heading\">
            <h2><span class=\"fa " . $config->get($keyname, 'icon') . " fa-3x\"></span></h2>
            <section>
                <a href=\"" . $config->get($keyname, 'url') . "\" target=\"_self\" title=\"Launch " . $config->get($keyname, 'name') . "!\"><button class=\"float\">Launch " . $config->get($keyname, 'name') . "</button></a>
            </section>
        </div>
     </div>
     </body></html>";
    if (empty($item)) $item = '';
    return $item;
}

// This method checks whether we can execute, if the directory is a git, and if git is installed
function can_git()
{
	if ((exec_enabled() == true) && (file_exists('.git'))) {
		$whereIsCommand = (PHP_OS == 'WINNT') ? 'where git' : 'which git'; 	// Establish the command for our OS
		$gitPath = shell_exec($whereIsCommand); 							// Find where git is
		$git = (empty($gitPath) ? false : true); 							// Make sure we have a path
		if ($git) {															// Double-check git is here and executable
			exec($gitPath . ' --version', $output);
			preg_match('#^(git version)#', current($output), $matches);
			$git = (empty($matches[0]) ? $gitPath : false);  				// If so, return path.  If not, return false.
		}
	} else {
		$git = false;
	}
	return $git;
}

// Can we execute commands?
function exec_enabled() {
    $disabled = explode(', ', ini_get('disable_functions'));
    return !in_array('exec', $disabled);
}

// This is where the JavaScript reads the contents of the secret file. This gets re-generated on each page load.
if (isset($_GET['get']) && $_GET['get'] == 'secret') {
        $secret = file_get_contents(SECRET) or die("Unable to open " . SECRET);
        echo $secret;
        die();
}
// Things wrapped inside this are protected by a secret hash.
if(isset($_GET['secret']) && $_GET['secret'] == file_get_contents(SECRET)) {
    // This lets us create a new secret when we leave the page.
    if (isset($_GET['set']) && $_GET['set'] == 'secret') {
            createSecret();
            die();
    }

    if (isset($_GET['get']) && $_GET['get'] == 'hash') {
        if (exec_enabled() == true) {
		$git = can_git();
            if ($git !== false) {
                $hash = 'unknown';
            } else {
                $hash = exec($git . ' log --pretty="%H" -n1 HEAD');
            }
        } else {
            $hash = 'noexec';
        }
        echo $hash;
        die();
    }

    if(isset($_GET['remove']) && $_GET['remove'] == "backup") {
        unlink('backup.ini.php');
        echo "deleted";
        die();
    }

    if(isset($_GET['action']) && $_GET['action'] == "update") {
        $sha = $_GET['sha'];
        $results = downloadUpdate($sha);
		if ($results === true) {
			echo $results;
			die();
		} else {
			$data = array('type' => 'error', 'message' => $results);
			header('HTTP/1.1 400 Bad Request');
			header('Content-Type: application/json; charset=UTF-8');
			echo json_encode($data);
			die();
		}
    }

    if(isset($_GET['action']) && $_GET['action'] == "branches") {
        $results = fetchBranches(true);
        echo $results;
        die();
    }

    if(isset($_GET['action']) && $_GET['action'] == "log") {
        echo log_contents();
        die();
    }

    if(isset($_GET['action']) && $_GET['action'] == "writeLog") {
        $msg = $_GET['msg'];
        if(isset($_GET['lvl'])) {
            $lvl = $_GET['lvl'];
            write_log($msg,$lvl);
        } else {
            write_log($msg);
        }
        die();
    }
}

// End protected get-calls
if(empty($_GET)) {
    createSecret();
}

// This downloads updates from git if available and able, otherwise, from zip.
function downloadUpdate($sha) {
	$git = can_git();
	if ($git !== false) {
		$result = exec('git status');
		$result = (preg_match('/working directory clean/',$result));
		if ($result !== true) {
			$result ='Install Failed!  Local instance has files that will interfer with git pull - please manually stash changes and try again.';
			write_log($result ,'E');
			return $result;
		}
		$result = exec('git pull');
		write_log('Updating via git, command result is ' . $result,'D');
		$result = (preg_match('/Updating/',$result));
		if ($result) {
			$mySha = exec('git rev-parse HEAD');
			$config = new Config_Lite(CONFIG);
			if (!preg_match('/about a specific subcommand/',$mySha)) { // Something went wrong with the command to get our SHA, fall back to using the passed value.
				$config->set('settings','sha',$mySha);
			} else {
				$config->set('settings','sha',$sha);
			}
			saveConfig($config);
		} else {
			$result = 'Install Failed!  An unknown error occurred attempting to update.  Please manually check git status and fix.';
		}
	} else {
		$result = false;
		$zipFile = "Muximux-".$sha. ".zip";
		$f = file_put_contents($zipFile, fopen("https://github.com/mescon/Muximux/archive/". $sha .".zip", 'r'), LOCK_EX);
		if(FALSE === $f) {
			$result = 'Install Failed!  An error occurred saving the update.  Please check directory permissions and try again.';
		} else {
			$zip = new ZipArchive;
			$res = $zip->open($zipFile);
			if ($res === TRUE) {
				$result = $zip->extractTo('./.stage');
				$zip->close();

				if ($result === TRUE) {
					cpy("./.stage/Muximux-".$sha, "./");
					deleteContent("./.stage");
					$gone = unlink($zipFile);
				} else {
					$result = 'Install Failed!  Unable to extract zip file.  Please check directory permissions and try again.';
				}
				$config = new Config_Lite(CONFIG);
				$config->set('settings','sha',$sha);
				saveConfig($config);
			} else {
				$result = 'Install Failed!  Unable to open zip file.  Check directory permissions and try again.';
			}
		}
	}
	if ($result === true) {
		write_log('Update Succeeded.','I');
	} else {
		write_log($result ,'E');
	}
    
    return $result;
}

// Copy a directory recursively - used to move updates after extraction
function cpy($source, $dest){
    if(is_dir($source)) {
        $dir_handle=opendir($source);
        while($file=readdir($dir_handle)){
            if($file!="." && $file!=".."){
                if(is_dir($source."/".$file)){
                    if(!is_dir($dest."/".$file)){
                        mkdir($dest."/".$file);
                    }
                    cpy($source."/".$file, $dest."/".$file);
                } else {
                    copy($source."/".$file, $dest."/".$file);
                }
            }
        }
        closedir($dir_handle);
    } else {
        copy($source, $dest);
    }
}

// Recursively delete the contents of a directory
function deleteContent($path){
    try{
        $iterator = new DirectoryIterator($path);
        foreach ( $iterator as $fileinfo ) {
        if($fileinfo->isDot())continue;
        if($fileinfo->isDir()){
            if(deleteContent($fileinfo->getPathname()))
                @rmdir($fileinfo->getPathname());
            }
            if($fileinfo->isFile()){
                @unlink($fileinfo->getPathname());
                }
        }
    } catch ( Exception $e ){
        // write log
        return false;
    }
    return true;
}

// This is used by our login script to determine session state 
function is_session_started() {
    if ( php_sapi_name() !== 'cli' ) {
        if ( version_compare(phpversion(), '5.4.0', '>=') ) {
            return session_status() === PHP_SESSION_ACTIVE ? TRUE : FALSE;
        } else {
            return session_id() === '' ? FALSE : TRUE;
        }
    }
    return FALSE;
}

// This might be excessive for just grabbing one theme value from CSS,
// but if we ever wanted to make a full theme editor, it could be handy.

function parseCSS($file,$searchSelector,$searchAttribute){
    $css = file_get_contents($file);
    preg_match_all( '/(?ims)([a-z0-9\s\.\:#_\-@,]+)\{([^\}]*)\}/', $css, $arr);
    $result = false;
    foreach ($arr[0] as $i => $x){
        $selector = trim($arr[1][$i]);
        if ($selector == $searchSelector) {
            $rules = explode(';', trim($arr[2][$i]));
            $rules_arr = array();
            foreach ($rules as $strRule){
                if (!empty($strRule)){
                    $rule = explode(":", $strRule);
                    if (trim($rule[0]) == $searchAttribute) {
                        $result = trim($rule[1]);
                    }
                }
            }
        }
    }
    return $result;
}

// Appends lines to file and makes sure the file doesn't grow too much
// You can supply a level, which should be a one-letter code (E for error, D for debug, I for information)
// If a level is not supplied, it will be assumed to be Informative.

function write_log($text,$level=null) {
    if ($level === null) {
        $level = 'I';
    }
    $filename = 'muximux.log';
    $text = $level .'/'. date(DATE_RFC2822) . ': ' . htmlspecialchars($text) . PHP_EOL;
    if (!file_exists($filename)) { touch($filename); chmod($filename, 0666); }
    if (filesize($filename) > 2*1024*1024) {
        $filename2 = "$filename.old";
        if (file_exists($filename2)) unlink($filename2);
        rename($filename, $filename2);
        touch($filename); chmod($filename,0666);
    }
    if (!is_writable($filename)) die("<p>\nCannot open log file ($filename)");
    if (!$handle = fopen($filename, 'a')) die("<p>\nCannot open file ($filename)");
    if (fwrite($handle, $text) === FALSE) die("<p>\nCannot write to file ($filename)");
    fclose($handle);
}

// Try to save our config and rewrite the header
function saveConfig($inConfig) {
    try {
        $inConfig->save();
    } catch (Config_Lite_Exception $e) {
        echo "\n" . 'Exception Message: ' . $e->getMessage();
    write_log('Error saving configuration.','E');
    }
    $cache_new = "; <?php die(\"Access denied\"); ?>"; // Adds this to the top of the config so that PHP kills the execution if someone tries to request the config-file remotely.
    $file = CONFIG; // the file to which $cache_new gets prepended
    $handle = openFile($file, "r+");
    $len = strlen($cache_new);
    $final_len = filesize($file) + $len;
    $cache_old = fread($handle, $len);
    rewind($handle);
    $i = 1;
    while (ftell($handle) < $final_len) {
        fwrite($handle, $cache_new);
        $cache_new = $cache_old;
        $cache_old = fread($handle, $len);
        fseek($handle, $i * $len);
        $i++;
    }
}