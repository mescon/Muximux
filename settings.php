<?php
/**
 * Created by PhpStorm.
 * User: synik
 * Date: 1/18/2016
 * Time: 9:19 PM
 */
require __DIR__ . '/vendor/autoload.php';

if (sizeof($_POST) > 0)
    write_ini();
else
    parse_ini();

function write_ini()
{
        unlink('config.ini.php');

    $config = new Config_Lite('config.ini.php');
    foreach ($_POST as $parameter => $value) {
        $splitParameter = explode('-', $parameter);
            if ($value == "on")
                $value = "true";
            $config->set($splitParameter[0], $splitParameter[1], $value);
    }
    // save object to file
    try {
        $config->save();
    } catch (Config_Lite_Exception $e) {
        echo "\n", 'Exception Message: ', $e->getMessage();
    } finally {
        echo true;
    }
}


function parse_ini()
{
    $config = new Config_Lite('config.ini.php');
    $iconVal = array('fa fa-television', 'fa fa-download','fa fa-server','fa fa-play-circle','fa fa-tint','fa fa-globe','glyphicon glyphicon-calendar', 'glyphicon glyphicon-dashboard',
        'glyphicon glyphicon-bullhorn',  'glyphicon glyphicon-search','glyphicon glyphicon-headphones');

    $pageOutput = "<form>";

    $pageOutput .= "<div class='applicationContainer' style='cursor: default;'><strong>General</strong><br>Title: <input type='text' class='general-value' name='general-title' value='" . $config->get('general', 'title') . "'>";
    $pageOutput .= "<div>Enable Dropdown: <input class='general-value' name='general-enabledropdown' type='checkbox' ";
    if ($config->get('general', 'enabledropdown') == true)
        $pageOutput .= "checked></div></div><br><br>";
    else
        $pageOutput .= "></div></div><br>";

    $pageOutput .= "<input type='hidden' class='settings-value' name='settings-enabled' value='true'>" .
        "<input type='hidden' class='settings-value' name='settings-default' value='false'>" .
        "<input type='hidden' class='settings-value' name='settings-name' value='Settings'>" .
        "<input type='hidden' class='settings-value' name='settings-url' value='settings.php'>" .
        "<input type='hidden' class='settings-value' name='settings-landingpage' value='false'>" .
        "<input type='hidden' class='settings-value' name='settings-icon' value='fa fa-server'>" .
        "<input type='hidden' class='settings-value' name='settings-dd' value='true'>";

    $pageOutput .= "<div id='sortable'>";
    foreach ($config as $section => $name) {
        if (is_array($name) && $section != "settings" && $section != "general") {
            $pageOutput .= "<div class='applicationContainer' id='" . $section . "'><span class='bars fa fa-bars'></span>";
            foreach ($name as $key => $val) {
                if ($key == "url")
                    $pageOutput .= "<div>URL:<input class='" . $section . "-value' name='" . $section . "-" . $key . "' type='text' value='" . $val . "'></div>";
                else if ($key == "name") {
                    $pageOutput .= "<div>Name:<input class='appName " . $section . "-value' was='" . $section . "' name='" . $section . "-" . $key . "' type='text' value='" . $val . "'></div>";
                } else if ($key == "icon") {
                    $pageOutput .= "<div>Icon:<select class='iconDD " . $section . "-value' name='" . $section . "-" . $key . "'>";
                    foreach ($iconVal as $icon) {
                        $pageOutput .= "<option value='" . $icon . "'" . ($val == $icon ? " selected>" : ">") . explode(' ',$icon)[1] . "</option>";
                    }
                    $pageOutput .= "</select><span class='example_icon'></span></div>";
                } elseif ($key == "default") {
                    $pageOutput .= "<div>Default:<input type='radio' class='radio " . $section . "-value' name='" . $section . "-" . $key . "'";
                    if ($val == "true")
                        $pageOutput .= " checked></div>";
                    else
                        $pageOutput .= "></div>";
                } else {
                    $pageOutput .= "<div>$key:<input class='checkbox " . $section . "-value' name='" . $section . "-" . $key . "' type='checkbox' ";
                    if ($val == "true")
                        $pageOutput .= " checked></div>";
                    else
                        $pageOutput .= "></div>";
                }
            }

            $pageOutput .= "<button type='button' class='removeButton btn btn-danger btn-xs' value='Remove' id='remove-" . $section . "'>Remove</button></div>"; //Put this back to the left when ajax is ready -- <input type='button' class='saveButton' value='Save' id='save-" . $section . "'>
        }
    }
    $pageOutput .= "</div><div class='center' id='addApplicationButton'>
                    <button type='button' class='btn btn-primary btn-md' id='addApplication'>Add new</button>
                    <div id='saved'>Saved!</div><div id='removed' class='hidden'></div></form>";
    return $pageOutput;
}