package com.cfdata.web;

import android.annotation.SuppressLint;
import android.Manifest;
import android.app.Activity;
import android.app.AlertDialog;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.res.Configuration;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.os.Handler;
import android.os.Looper;
import android.provider.Settings;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.view.WindowInsetsController;
import android.view.WindowManager;
import android.webkit.JavascriptInterface;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.TextView;
import android.widget.Toast;

import java.io.File;
import java.io.IOException;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class MainActivity extends Activity {
    private static final int PORT = 13335;
    private static final String BACKEND_LIBRARY = "libcfdata.so";
    private static final int REQUEST_FILE_CHOOSER = 1002;
    private static final int REQUEST_STORAGE_PERMISSION = 1003;

    private final ExecutorService executor = Executors.newSingleThreadExecutor();
    private final Handler mainHandler = new Handler(Looper.getMainLooper());
    private WebView webView;
    private FrameLayout rootView;
    private View loadingView;
    private TextView loadingTitle;
    private TextView loadingMessage;
    private ProgressBar loadingSpinner;
    private Process backendProcess;
    private File dataDirectory;
    private boolean backendStarting;
    private ValueCallback<Uri[]> filePathCallback;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        rootView = new FrameLayout(this);
        webView = new WebView(this);
        rootView.addView(webView, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        loadingView = createLoadingView();
        rootView.addView(loadingView, new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        setContentView(rootView, new ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        applySystemBarInsets(rootView);
        applyStatusBarTheme();
        configureWebView();
        requestStorageAndStartBackend();
    }

    private boolean isDarkMode() {
        int nightMode = getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK;
        return nightMode == Configuration.UI_MODE_NIGHT_YES;
    }

    private void applyStatusBarTheme() {
        setStatusBarAppearance(isDarkMode());
    }

    private void setStatusBarAppearance(boolean dark) {
        int bgColor = dark ? Color.rgb(15, 23, 42) : Color.rgb(246, 248, 251);
        getWindow().addFlags(WindowManager.LayoutParams.FLAG_DRAWS_SYSTEM_BAR_BACKGROUNDS);
        getWindow().setStatusBarColor(bgColor);
        getWindow().setNavigationBarColor(bgColor);
        rootView.setBackgroundColor(bgColor);
        View decor = getWindow().getDecorView();
        int flags = decor.getSystemUiVisibility();
        if (dark) {
            decor.setSystemUiVisibility(flags & ~View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR);
        } else {
            decor.setSystemUiVisibility(flags | View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR);
        }
    }

    private View createLoadingView() {
        boolean dark = isDarkMode();
        FrameLayout overlay = new FrameLayout(this);
        overlay.setBackgroundColor(dark ? Color.rgb(15, 23, 42) : Color.rgb(246, 248, 251));

        LinearLayout card = new LinearLayout(this);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setGravity(Gravity.CENTER_HORIZONTAL);
        card.setPadding(dp(28), dp(30), dp(28), dp(28));
        card.setElevation(dp(8));

        GradientDrawable cardBackground = new GradientDrawable();
        cardBackground.setColor(dark ? Color.rgb(30, 41, 59) : Color.WHITE);
        cardBackground.setCornerRadius(dp(22));
        cardBackground.setStroke(dp(1), dark ? Color.argb(160, 51, 65, 85) : Color.argb(160, 226, 232, 240));
        card.setBackground(cardBackground);

        ImageView logo = new ImageView(this);
        logo.setImageResource(getApplicationInfo().icon);
        logo.setAdjustViewBounds(true);
        logo.setScaleType(ImageView.ScaleType.FIT_CENTER);
        LinearLayout.LayoutParams logoParams = new LinearLayout.LayoutParams(dp(64), dp(64));
        card.addView(logo, logoParams);

        loadingTitle = new TextView(this);
        loadingTitle.setText("CFData");
        loadingTitle.setTextColor(dark ? Color.rgb(241, 245, 249) : Color.rgb(31, 41, 55));
        loadingTitle.setTextSize(24);
        loadingTitle.setTypeface(Typeface.DEFAULT_BOLD);
        loadingTitle.setGravity(Gravity.CENTER);
        LinearLayout.LayoutParams titleParams = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        titleParams.topMargin = dp(18);
        card.addView(loadingTitle, titleParams);

        loadingMessage = new TextView(this);
        loadingMessage.setText("正在启动本地服务...");
        loadingMessage.setTextColor(dark ? Color.rgb(148, 163, 184) : Color.rgb(100, 116, 139));
        loadingMessage.setTextSize(14);
        loadingMessage.setGravity(Gravity.CENTER);
        LinearLayout.LayoutParams messageParams = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        messageParams.topMargin = dp(8);
        card.addView(loadingMessage, messageParams);

        loadingSpinner = new ProgressBar(this);
        loadingSpinner.setIndeterminate(true);
        LinearLayout.LayoutParams spinnerParams = new LinearLayout.LayoutParams(dp(36), dp(36));
        spinnerParams.topMargin = dp(22);
        card.addView(loadingSpinner, spinnerParams);

        FrameLayout.LayoutParams cardParams = new FrameLayout.LayoutParams(Math.min(dp(320), getResources().getDisplayMetrics().widthPixels - dp(48)), ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.CENTER);
        overlay.addView(card, cardParams);
        return overlay;
    }

    private void applySystemBarInsets(View view) {
        view.setOnApplyWindowInsetsListener((v, insets) -> {
            v.setPadding(0, insets.getSystemWindowInsetTop(), 0, insets.getSystemWindowInsetBottom());
            return insets;
        });
        view.requestApplyInsets();
    }

    @SuppressLint({"SetJavaScriptEnabled", "AddJavascriptInterface"})
    private void configureWebView() {
        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setAllowFileAccess(true);
        settings.setAllowContentAccess(true);
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        webView.addJavascriptInterface(new AndroidBridge(), "CFDataAndroid");
        webView.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                Uri uri = request.getUrl();
                if (uri != null && ("http".equals(uri.getScheme()) || "https".equals(uri.getScheme()))) {
                    String host = uri.getHost();
                    if ("127.0.0.1".equals(host) || "localhost".equals(host)) {
                        return false;
                    }
                    startActivity(new Intent(Intent.ACTION_VIEW, uri));
                    return true;
                }
                return false;
            }
        });
        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public boolean onShowFileChooser(WebView view, ValueCallback<Uri[]> callback, FileChooserParams params) {
                if (filePathCallback != null) {
                    filePathCallback.onReceiveValue(null);
                }
                filePathCallback = callback;
                Intent intent = params.createIntent();
                intent.setType("*/*");
                intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[]{
                        "text/plain",
                        "text/csv",
                        "text/comma-separated-values",
                        "application/csv",
                        "application/vnd.ms-excel",
                        "application/octet-stream"
                });
                try {
                    startActivityForResult(intent, REQUEST_FILE_CHOOSER);
                } catch (ActivityNotFoundException e) {
                    filePathCallback = null;
                    Toast.makeText(MainActivity.this, "没有可用的文件选择器", Toast.LENGTH_LONG).show();
                    return false;
                }
                return true;
            }
        });
    }

    private boolean hasDataStoragePermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            return Environment.isExternalStorageManager();
        }
        return checkSelfPermission(Manifest.permission.WRITE_EXTERNAL_STORAGE) == PackageManager.PERMISSION_GRANTED;
    }

    private void requestStorageAndStartBackend() {
        if (hasDataStoragePermission()) {
            startBackend();
            return;
        }
        setLoadingMessage("需要存储授权，才能保存配置、扫描结果和导出文件");
        new AlertDialog.Builder(this)
                .setTitle("授权保存应用文件")
                .setMessage("应用文件将统一保存在 Download/cfdata-web。Android 11 及以上请开启本应用的“所有文件访问权限”；凭据仍以加密文件保存。")
                .setPositiveButton("去授权", (dialog, which) -> requestDataStoragePermission())
                .setNegativeButton("暂不授权", (dialog, which) -> showStoragePermissionError())
                .setOnCancelListener(dialog -> showStoragePermissionError())
                .show();
    }

    private void requestDataStoragePermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            Intent intent = new Intent(Settings.ACTION_MANAGE_APP_ALL_FILES_ACCESS_PERMISSION,
                    Uri.parse("package:" + getPackageName()));
            try {
                startActivityForResult(intent, REQUEST_STORAGE_PERMISSION);
            } catch (ActivityNotFoundException e) {
                try {
                    startActivityForResult(new Intent(Settings.ACTION_MANAGE_ALL_FILES_ACCESS_PERMISSION), REQUEST_STORAGE_PERMISSION);
                } catch (ActivityNotFoundException | SecurityException unavailable) {
                    showStoragePermissionError();
                }
            } catch (SecurityException e) {
                showStoragePermissionError();
            }
        } else {
            requestPermissions(new String[]{Manifest.permission.READ_EXTERNAL_STORAGE,
                    Manifest.permission.WRITE_EXTERNAL_STORAGE}, REQUEST_STORAGE_PERMISSION);
        }
    }

    private void showStoragePermissionError() {
        showStartupError("需要存储权限", "尚未获得 Download/cfdata-web 的访问权限，未启动服务。点击此页面可重新授权。");
        loadingView.setOnClickListener(view -> requestStorageAndStartBackend());
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode == REQUEST_STORAGE_PERMISSION) {
            if (hasDataStoragePermission()) {
                startBackend();
            } else {
                showStoragePermissionError();
            }
        }
    }

    private void startBackend() {
        if (backendStarting) {
            return;
        }
        backendStarting = true;
        loadingView.setOnClickListener(null);
        loadingSpinner.setVisibility(View.VISIBLE);
        loadingTitle.setText("CFData-WEB");
        executor.execute(() -> {
            try {
                setLoadingMessage("正在准备本地服务...");
                dataDirectory = new File(Environment.getExternalStoragePublicDirectory(Environment.DIRECTORY_DOWNLOADS), "cfdata-web");
                AppDataFiles.prepareDirectory(dataDirectory);
                AppDataFiles.migrateLegacyFiles(getFilesDir(), dataDirectory);
                File backend = prepareBackendBinary();
                ProcessBuilder builder = new ProcessBuilder(
                        backend.getAbsolutePath(),
                        "-host", "127.0.0.1",
                        "-port", String.valueOf(PORT)
                );
                builder.directory(dataDirectory);
                builder.environment().put("CFDATA_DATA_DIR", dataDirectory.getAbsolutePath());
                builder.environment().put("TMPDIR", dataDirectory.getAbsolutePath());
                builder.redirectErrorStream(true);
                builder.redirectOutput(ProcessBuilder.Redirect.appendTo(new File(dataDirectory, "cfdata-backend.log")));
                backendProcess = builder.start();
                setLoadingMessage("正在连接本地服务...");
                waitForBackend();
                mainHandler.post(() -> {
                    setLoadingMessage("正在加载界面...");
                    webView.loadUrl("http://127.0.0.1:" + PORT);
                });
            } catch (Exception e) {
                if (backendProcess != null) {
                    backendProcess.destroy();
                }
                mainHandler.post(() -> {
                    backendStarting = false;
                    showStartupError("启动失败", "无法准备 Download/cfdata-web 或启动本地服务，请检查存储权限后重新打开应用\n" + e.getMessage());
                });
            }
        });
    }

    private void setLoadingMessage(String message) {
        mainHandler.post(() -> {
            if (loadingMessage != null) {
                loadingMessage.setText(message);
            }
        });
    }

    private void hideLoadingView() {
        View view = loadingView;
        if (view != null) {
            view.animate().alpha(0f).setDuration(180).withEndAction(() -> view.setVisibility(View.GONE)).start();
        }
    }

    private void showStartupError(String title, String message) {
        if (loadingView != null) {
            loadingView.setVisibility(View.VISIBLE);
            loadingView.setAlpha(1f);
        }
        if (loadingTitle != null) {
            loadingTitle.setText(title);
            loadingTitle.setTextColor(Color.rgb(185, 28, 28));
        }
        if (loadingMessage != null) {
            loadingMessage.setText(message);
        }
        if (loadingSpinner != null) {
            loadingSpinner.setVisibility(View.GONE);
        }
        Toast.makeText(this, title + ": " + message, Toast.LENGTH_LONG).show();
    }

    private File prepareBackendBinary() throws IOException {
        File nativeBackend = new File(getApplicationInfo().nativeLibraryDir, BACKEND_LIBRARY);
        if (nativeBackend.isFile()) {
            return nativeBackend;
        }
        throw new IOException("未找到内置后端: " + nativeBackend.getAbsolutePath());
    }

    private void waitForBackend() throws InterruptedException {
        long deadline = System.currentTimeMillis() + 15000;
        while (System.currentTimeMillis() < deadline) {
            if (isBackendReady()) {
                injectExportBridge();
                return;
            }
            Thread.sleep(300);
        }
        throw new IllegalStateException("本地服务启动超时");
    }

    private boolean isBackendReady() {
        HttpURLConnection connection = null;
        try {
            URL url = new URL("http://127.0.0.1:" + PORT + "/favicon.png");
            connection = (HttpURLConnection) url.openConnection();
            connection.setConnectTimeout(500);
            connection.setReadTimeout(500);
            return connection.getResponseCode() < 500;
        } catch (IOException ignored) {
            return false;
        } finally {
            if (connection != null) {
                connection.disconnect();
            }
        }
    }

    private void injectExportBridge() {
        mainHandler.post(() -> webView.setWebViewClient(new WebViewClient() {
            @Override
            public void onPageFinished(WebView view, String url) {
                super.onPageFinished(view, url);
                String script = "(function(){"
                        + "if(!window.CFDataAndroid||window.__cfdataAndroidExport)return;"
                        + "window.__cfdataAndroidExport=true;"
                        + "var old=window.downloadFile;"
                        + "window.downloadFile=function(content,nameBase,ext){"
                        + "var ts=new Date().toISOString().replace(/[-:T]/g,'').split('.')[0];"
                        + "var name=(nameBase||'cfdata-results')+'_'+ts+'.'+(ext||'txt');"
                        + "window.CFDataAndroid.saveTextFile(name,String(content||''));"
                        + "};"
                        + "})();";
                view.evaluateJavascript(script, null);
                hideLoadingView();
            }

            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                Uri uri = request.getUrl();
                if (uri != null && ("http".equals(uri.getScheme()) || "https".equals(uri.getScheme()))) {
                    String host = uri.getHost();
                    if ("127.0.0.1".equals(host) || "localhost".equals(host)) {
                        return false;
                    }
                    startActivity(new Intent(Intent.ACTION_VIEW, uri));
                    return true;
                }
                return false;
            }
        }));
    }

    public class AndroidBridge {
        @JavascriptInterface
        public boolean isDarkMode() {
            int nightMode = getResources().getConfiguration().uiMode & Configuration.UI_MODE_NIGHT_MASK;
            return nightMode == Configuration.UI_MODE_NIGHT_YES;
        }

        @JavascriptInterface
        public void setDarkModeStatusBar(boolean dark) {
            mainHandler.post(() -> setStatusBarAppearance(dark));
        }

        @JavascriptInterface
        public void saveTextFile(String fileName, String content) {
            executor.execute(() -> {
                try {
                    if (dataDirectory == null || !hasDataStoragePermission()) {
                        throw new IOException("存储权限未授权，请重新打开应用并授权");
                    }
                    File saved = AppDataFiles.writeExport(dataDirectory, fileName, content);
                    mainHandler.post(() -> Toast.makeText(MainActivity.this, "已保存: " + saved.getAbsolutePath(), Toast.LENGTH_LONG).show());
                } catch (IOException | SecurityException e) {
                    mainHandler.post(() -> Toast.makeText(MainActivity.this, "保存失败: " + e.getMessage(), Toast.LENGTH_LONG).show());
                }
            });
        }
    }

    private int dp(int value) {
        return Math.round(value * getResources().getDisplayMetrics().density);
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == REQUEST_STORAGE_PERMISSION) {
            if (hasDataStoragePermission()) {
                startBackend();
            } else {
                showStoragePermissionError();
            }
            return;
        }
        if (requestCode == REQUEST_FILE_CHOOSER) {
            if (filePathCallback == null) {
                return;
            }
            Uri[] result = WebChromeClient.FileChooserParams.parseResult(resultCode, data);
            filePathCallback.onReceiveValue(result);
            filePathCallback = null;
        }
    }

    @Override
    public void onBackPressed() {
        if (webView != null && webView.canGoBack()) {
            webView.goBack();
            return;
        }
        super.onBackPressed();
    }

    @Override
    protected void onDestroy() {
        if (backendProcess != null) {
            backendProcess.destroy();
        }
        executor.shutdownNow();
        if (webView != null) {
            webView.destroy();
        }
        super.onDestroy();
    }
}
