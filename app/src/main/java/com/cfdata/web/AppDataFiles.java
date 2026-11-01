package com.cfdata.web;

import java.io.File;
import java.io.IOException;
import java.io.OutputStream;
import java.nio.channels.Channels;
import java.nio.channels.FileChannel;
import java.nio.charset.StandardCharsets;
import java.nio.file.FileAlreadyExistsException;
import java.nio.file.Files;
import java.nio.file.StandardOpenOption;

final class AppDataFiles {
    private static final String[] LEGACY_FILES = {
            "huawei-dns.private", "web-state.private", "latest-results.json", "cfdata-config.json",
            "cfdata-config.json.bak", "locations.json", "ips-v4.txt", "ips-v6.txt",
            "GeoLite2-ASN.mmdb", "cfdata-debug.log", "cfdata-backend.log"
    };

    static void prepareDirectory(File directory) throws IOException {
        Files.createDirectories(directory.toPath());
        File probe = File.createTempFile(".cfdata-write-check-", ".tmp", directory);
        Files.delete(probe.toPath());
    }

    static void migrateLegacyFiles(File legacyDirectory, File directory) throws IOException {
        File[] files = legacyDirectory.listFiles();
        if (files == null) {
            return;
        }
        for (File source : files) {
            String name = source.getName();
            boolean migrate = name.endsWith(".csv") || name.endsWith(".txt");
            for (String known : LEGACY_FILES) {
                migrate |= known.equals(name);
            }
            if (!migrate || !source.isFile() || new File(directory, name).exists()) {
                continue;
            }
            File destination = new File(directory, name);
            boolean created = false;
            // shortcut: shared storage lacks portable atomic no-replace rename; interruption may leave a partial new copy, never replace an old file.
            try (FileChannel out = FileChannel.open(destination.toPath(), StandardOpenOption.CREATE_NEW, StandardOpenOption.WRITE)) {
                created = true;
                Files.copy(source.toPath(), Channels.newOutputStream(out));
                out.force(true);
            } catch (FileAlreadyExistsException ignored) {
                // A destination created during migration takes priority over the old copy.
            } catch (IOException | RuntimeException error) {
                if (created) {
                    removeFailedCopy(destination, error);
                }
                throw error;
            }
        }
    }

    static File writeExport(File directory, String fileName, String content) throws IOException {
        String name = fileName == null ? "" : fileName.replaceAll("[\\\\/:*?\"<>|\\p{Cntrl}]", "_").trim();
        if (name.isEmpty() || name.equals(".") || name.equals("..")) {
            name = "cfdata-results.txt";
        }
        int dot = name.lastIndexOf('.');
        String base = dot > 0 ? name.substring(0, dot) : name;
        String extension = dot > 0 ? name.substring(dot) : "";
        for (int index = 0; index < 10000; index++) {
            File destination = new File(directory, index == 0 ? name : base + "_" + index + extension);
            boolean created = false;
            try (FileChannel channel = FileChannel.open(destination.toPath(), StandardOpenOption.CREATE_NEW, StandardOpenOption.WRITE)) {
                created = true;
                OutputStream out = Channels.newOutputStream(channel);
                out.write(new byte[]{(byte) 0xEF, (byte) 0xBB, (byte) 0xBF});
                out.write((content == null ? "" : content).getBytes(StandardCharsets.UTF_8));
                channel.force(true);
                return destination;
            } catch (FileAlreadyExistsException ignored) {
                // Repeated exports must never replace an earlier result.
            } catch (IOException | RuntimeException error) {
                if (created) {
                    removeFailedCopy(destination, error);
                }
                throw error;
            }
        }
        throw new IOException("Too many exports with the same filename");
    }

    private static void removeFailedCopy(File file, Exception original) {
        try {
            Files.deleteIfExists(file.toPath());
        } catch (IOException | SecurityException cleanup) {
            original.addSuppressed(cleanup);
        }
    }
}
